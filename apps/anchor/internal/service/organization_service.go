package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"

	"anchor/internal/domain/organization"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
)

const lockKeyCreateWithMember = "organization:create-with-member:%s:%s"

type OrganizationService interface {
	Find(ctx context.Context, input organization.FindOrganizationInput) (
		*organization.Organization, error,
	)
	Create(
		ctx context.Context, input organization.CreateOrganizationInput,
	) (organization.Organization, error)
	CreateWithMember(
		ctx context.Context, input organization.CreateOrganizationWithMemberInput,
	) (organization.OrganizationWithMemberResult, error)
	Update(
		ctx context.Context, input organization.UpdateOrganizationInput,
	) (organization.Organization, error)
	Delete(ctx context.Context, input organization.DeleteOrganizationInput) error
	Search(
		ctx context.Context, input organization.SearchProductOrganizationsInput,
	) (search.Result[organization.Organization], error)
}

type organizationService struct {
	organizationRepo  repository.OrganizationRepository
	orgMembershipRepo repository.OrganizationMembershipRepository
	productUserRepo   repository.ProductUserRepository
	productRoleRepo   repository.ProductRoleRepository
	lock              *pglock.Client
	db                *sql.DB
	logger            zerolog.Logger
}

func NewOrganizationService(
	organizationRepo repository.OrganizationRepository,
	orgMembershipRepo repository.OrganizationMembershipRepository,
	productUserRepo repository.ProductUserRepository,
	productRoleRepo repository.ProductRoleRepository,
	lock *pglock.Client,
	db *sql.DB,
	logger zerolog.Logger,
) OrganizationService {
	return &organizationService{
		organizationRepo:  organizationRepo,
		orgMembershipRepo: orgMembershipRepo,
		productUserRepo:   productUserRepo,
		productRoleRepo:   productRoleRepo,
		lock:              lock,
		db:                db,
		logger:            logger.With().Str("component", "organization_service").Logger(),
	}
}

func (s *organizationService) Find(
	ctx context.Context, input organization.FindOrganizationInput,
) (*organization.Organization, error) {
	logger := s.logger.With().Str("operation", "Find").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}

	org, err := s.organizationRepo.FindByID(ctx, input.ProductID, input.OrganizationID, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Err(err).
			Msg("failed to find organization")
		return nil, err
	}
	return org, nil
}

func (s *organizationService) Create(
	ctx context.Context, input organization.CreateOrganizationInput,
) (organization.Organization, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return organization.Organization{}, err
	}

	org := organization.Organization{
		ProductID:   input.ProductID,
		Name:        input.Name,
		Description: input.Description,
	}
	org.GenerateID()

	created, err := s.organizationRepo.Create(ctx, org, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to create organization")
		return organization.Organization{}, err
	}

	logger.Info().
		Str("organization_id", created.ID).
		Str("product_id", input.ProductID).
		Str("name", input.Name).
		Msg("organization created")

	return created, nil
}

func (s *organizationService) CreateWithMember(
	ctx context.Context, input organization.CreateOrganizationWithMemberInput,
) (organization.OrganizationWithMemberResult, error) {
	logger := s.logger.With().Str("operation", "CreateWithMember").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return organization.OrganizationWithMemberResult{}, err
	}

	existingMemberships, err := s.orgMembershipRepo.FindByProductUserID(
		ctx, input.ProductID, input.ProductUserID, false, nil,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Msg("failed to check existing memberships for idempotency")
		return organization.OrganizationWithMemberResult{}, err
	}
	if len(existingMemberships) > 0 {
		existing := existingMemberships[0]

		org, errGetOrg := s.organizationRepo.FindByID(ctx, input.ProductID, existing.OrganizationID, nil)
		if errGetOrg != nil {
			return organization.OrganizationWithMemberResult{}, errGetOrg
		}
		if org == nil {
			return organization.OrganizationWithMemberResult{}, toolkit.ErrNotFound
		}

		membership, errGetOrg := s.orgMembershipRepo.FindByOrgIDAndUserID(
			ctx, input.ProductID, existing.OrganizationID, input.ProductUserID, false, nil,
		)
		if errGetOrg != nil {
			return organization.OrganizationWithMemberResult{}, errGetOrg
		}
		if membership == nil {
			return organization.OrganizationWithMemberResult{}, toolkit.ErrNotFound
		}

		logger.Info().
			Str("product_id", input.ProductID).
			Str("organization_id", org.ID).
			Str("product_user_id", input.ProductUserID).
			Msg("CreateWithMember: returning existing org+membership (idempotent)")

		return organization.OrganizationWithMemberResult{
			Organization: *org,
			Membership:   *membership,
			WasExisting:  true,
		}, nil
	}

	// No existing membership — acquire the lock and create the org + founding membership atomically.
	lockKey := fmt.Sprintf(lockKeyCreateWithMember, input.ProductID, input.ProductUserID)

	var createdOrg organization.Organization
	var createdMembership organization.Membership

	acquired, err := s.lock.TryWithLock(ctx, lockKey, func(lockCtx context.Context, tx *sql.Tx) error {
		txOpts := &toolkit.DBOptions{Tx: tx}

		productUserEntity, lookupErr := s.productUserRepo.FindByProductIDAndID(
			lockCtx,
			input.ProductID,
			input.ProductUserID,
			txOpts,
		)
		if lookupErr != nil {
			logger.Error().Err(lookupErr).
				Str("product_id", input.ProductID).
				Str("product_user_id", input.ProductUserID).
				Msg("failed to verify product user")
			return lookupErr
		}
		if productUserEntity == nil {
			return toolkit.ErrNotFound
		}

		role, roleErr := s.productRoleRepo.FindByProductIDAndRoleID(
			lockCtx,
			input.ProductID,
			input.RoleID,
			txOpts,
		)
		if roleErr != nil {
			logger.Error().Err(roleErr).
				Str("product_id", input.ProductID).
				Str("role_id", input.RoleID).
				Msg("failed to verify product role")
			return roleErr
		}
		if role == nil {
			return NewRoleNotFoundError(input.RoleID)
		}

		org := organization.Organization{
			ProductID:   input.ProductID,
			Name:        input.Name,
			Description: input.Description,
		}
		org.GenerateID()

		createdOrg, err = s.organizationRepo.Create(lockCtx, org, txOpts)
		if err != nil {
			logger.Error().Err(err).
				Str("product_id", input.ProductID).
				Str("name", input.Name).
				Msg("failed to create organization")
			return err
		}

		createdMembership, err = s.orgMembershipRepo.Create(
			lockCtx, input.ProductID, createdOrg.ID, input.ProductUserID, input.RoleID, txOpts,
		)
		if err != nil {
			logger.Error().Err(err).
				Str("product_id", input.ProductID).
				Str("organization_id", createdOrg.ID).
				Str("product_user_id", input.ProductUserID).
				Msg("failed to create founding membership")
			return err
		}

		return nil
	})
	if err != nil {
		return organization.OrganizationWithMemberResult{}, err
	}
	if !acquired {
		logger.Warn().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Msg("create-with-member lock was busy; concurrent request in progress")
		return organization.OrganizationWithMemberResult{}, toolkit.ErrUnexpected
	}

	logger.Info().
		Str("product_id", input.ProductID).
		Str("organization_id", createdOrg.ID).
		Str("product_user_id", input.ProductUserID).
		Str("role_id", input.RoleID).
		Msg("organization created with founding member")

	return organization.OrganizationWithMemberResult{
		Organization: createdOrg,
		Membership:   createdMembership,
		WasExisting:  false,
	}, nil
}

func (s *organizationService) Update(
	ctx context.Context, input organization.UpdateOrganizationInput,
) (organization.Organization, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return organization.Organization{}, err
	}

	optOrg, err := s.organizationRepo.FindByID(
		ctx, input.ProductID, input.OrganizationID, nil,
	)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to find organization")
		return organization.Organization{}, err
	}
	if optOrg == nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Msg("organization not found for update")
		return organization.Organization{}, toolkit.ErrNotFound
	}

	org := *optOrg
	if input.Name != nil {
		org.Name = *input.Name
	}
	org.Description = input.Description

	updated, err := s.organizationRepo.Update(ctx, input.ProductID, org, nil)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to update organization")
		return organization.Organization{}, err
	}

	logger.Info().
		Str("organization_id", input.OrganizationID).
		Str("product_id", input.ProductID).
		Msg("organization updated")

	return updated, nil
}

func (s *organizationService) Delete(
	ctx context.Context, input organization.DeleteOrganizationInput,
) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return err
	}

	optOrg, err := s.organizationRepo.FindByID(
		ctx, input.ProductID, input.OrganizationID, nil,
	)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to find organization")
		return err
	}
	if optOrg == nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Msg("organization not found for deletion")
		return toolkit.ErrNotFound
	}

	err = s.organizationRepo.DeleteByID(ctx, input.ProductID, input.OrganizationID, nil)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to delete organization")
		return err
	}

	logger.Info().
		Str("organization_id", input.OrganizationID).
		Str("product_id", input.ProductID).
		Msg("organization deleted")

	return nil
}

func (s *organizationService) Search(
	ctx context.Context, input organization.SearchProductOrganizationsInput,
) (search.Result[organization.Organization], error) {
	logger := s.logger.With().Str("operation", "Search").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return search.Result[organization.Organization]{}, err
	}

	result, err := s.organizationRepo.SearchByProductID(ctx, input.ProductID, input.Request, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to search organizations")
		return search.Result[organization.Organization]{}, err
	}

	return result, nil
}

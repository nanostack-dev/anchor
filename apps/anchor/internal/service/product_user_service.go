package service

import (
	"context"
	"time"

	"anchor/internal/domain/product/user"

	"anchor/internal/repository"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"github.com/rs/zerolog"
)

type ProductUserService interface {
	Find(ctx context.Context, input user.FindProductUserInput) (*user.ProductUser, error)
	FindByExternalID(ctx context.Context, input user.FindProductUserByExternalIDInput) (*user.ProductUser, error)
	Create(ctx context.Context, input user.CreateProductUserInput) (user.ProductUser, error)
	Delete(ctx context.Context, input user.DeleteProductUserInput) error
	Search(
		ctx context.Context, input user.SearchProductUserInput,
	) (search.Result[user.ProductUser], error)
	ListUserOrganizations(
		ctx context.Context, input user.ListUserOrganizationsInput,
	) ([]user.OrganizationMembership, error)
	GetUserOrganization(
		ctx context.Context, input user.GetUserOrganizationInput,
	) (*user.OrganizationMembership, error)
}

var ErrProductUserEmailAlreadyExists = fault.Conflict(
	"PRODUCT_USER_EMAIL_ALREADY_EXISTS",
	"A product user with this email already exists in this product",
)

type productUserService struct {
	productUserRepo   repository.ProductUserRepository
	orgMembershipRepo repository.OrganizationMembershipRepository
	transactor        transactor.Transactor
	logger            zerolog.Logger
}

func NewProductUserService(
	productUserRepo repository.ProductUserRepository,
	orgMembershipRepo repository.OrganizationMembershipRepository,
	transactor transactor.Transactor,
	logger zerolog.Logger,
) ProductUserService {
	return &productUserService{
		productUserRepo:   productUserRepo,
		orgMembershipRepo: orgMembershipRepo,
		transactor:        transactor,
		logger:            logger.With().Str("component", "product_user_service").Logger(),
	}
}

func (s *productUserService) Find(
	ctx context.Context, input user.FindProductUserInput,
) (*user.ProductUser, error) {
	logger := s.logger.With().Str("operation", "Find").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	found, err := s.productUserRepo.FindByProductIDAndID(
		ctx, input.ProductID, input.ProductUserID,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Err(err).
			Msg("failed to find product user")
		return nil, err
	}

	return found.ToPtr(), nil
}

func (s *productUserService) Create(
	ctx context.Context, input user.CreateProductUserInput,
) (user.ProductUser, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := validateStruct(input); err != nil {
		return user.ProductUser{}, err
	}

	var createdUser user.ProductUser
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		existingUsers, err := s.productUserRepo.FindByProductID(txCtx, input.ProductID)
		if err != nil {
			logger.Error().
				Str("product_id", input.ProductID).
				Str("email", input.Email).
				Err(err).
				Msg("failed to check for duplicate email")
			return err
		}

		emailExists := functional.Slice(existingUsers).AnyMatch(func(existingUser user.ProductUser) bool {
			return existingUser.Email == input.Email
		})
		if emailExists {
			return ErrProductUserEmailAlreadyExists
		}

		productUser := user.ProductUser{
			ProductID: input.ProductID,
			Email:     input.Email,
			Name:      input.Name,
			Status:    input.Status,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		productUser.GenerateID()

		createdUser, err = s.productUserRepo.Create(txCtx, productUser)
		if err != nil {
			logger.Error().
				Str("product_id", input.ProductID).
				Str("email", input.Email).
				Err(err).
				Msg("failed to create product user")
			return err
		}

		logger.Info().
			Str("product_user_id", createdUser.ID).
			Str("product_id", input.ProductID).
			Str("email", input.Email).
			Msg("product user created successfully")

		return nil
	})
	return createdUser, err
}

func (s *productUserService) Delete(
	ctx context.Context, input user.DeleteProductUserInput,
) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := validateStruct(input); err != nil {
		return err
	}

	err := s.productUserRepo.DeleteByID(ctx, input.ProductID, input.ProductUserID)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Err(err).
			Msg("failed to delete product user")
		return err
	}

	logger.Info().
		Str("product_user_id", input.ProductUserID).
		Str("product_id", input.ProductID).
		Msg("product user deleted successfully")

	return nil
}

func (s *productUserService) Search(
	ctx context.Context, input user.SearchProductUserInput,
) (search.Result[user.ProductUser], error) {
	logger := s.logger.With().Str("operation", "Search").Logger()

	if err := validateStruct(input); err != nil {
		return search.Result[user.ProductUser]{}, err
	}

	result, err := s.productUserRepo.SearchByProductID(ctx, input.ProductID, input.Request)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to search product users")
		return search.Result[user.ProductUser]{}, err
	}

	logger.Debug().
		Str("product_id", input.ProductID).
		Int("total_count", int(result.Total)).
		Msg("product users search completed")

	return result, nil
}

func (s *productUserService) FindByExternalID(
	ctx context.Context, input user.FindProductUserByExternalIDInput,
) (*user.ProductUser, error) {
	logger := s.logger.With().Str("operation", "FindByExternalID").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	found, err := s.productUserRepo.FindByExternalID(
		ctx, input.ProductID, input.ExternalID,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("external_id", input.ExternalID).
			Err(err).
			Msg("failed to find product user by external ID")
		return nil, err
	}

	return found.ToPtr(), nil
}

func (s *productUserService) ListUserOrganizations(
	ctx context.Context, input user.ListUserOrganizationsInput,
) ([]user.OrganizationMembership, error) {
	logger := s.logger.With().Str("operation", "ListUserOrganizations").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	// Verify the product user exists
	foundUser, err := s.productUserRepo.FindByProductIDAndID(
		ctx, input.ProductID, input.ProductUserID,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Err(err).
			Msg("failed to verify product user exists")
		return nil, err
	}
	if foundUser.IsAbsent() {
		return nil, fault.ErrNotFound
	}

	memberships, err := s.orgMembershipRepo.FindByProductUserID(
		ctx, input.ProductID, input.ProductUserID, input.IncludePermissions,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Err(err).
			Msg("failed to list user organizations")
		return nil, err
	}

	return memberships, nil
}

func (s *productUserService) GetUserOrganization(
	ctx context.Context, input user.GetUserOrganizationInput,
) (*user.OrganizationMembership, error) {
	logger := s.logger.With().Str("operation", "GetUserOrganization").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	// Verify the product user exists
	foundUser, err := s.productUserRepo.FindByProductIDAndID(
		ctx, input.ProductID, input.ProductUserID,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Err(err).
			Msg("failed to verify product user exists")
		return nil, err
	}
	if foundUser.IsAbsent() {
		return nil, fault.ErrNotFound
	}

	found, err := s.orgMembershipRepo.FindByProductUserIDAndOrgID(
		ctx, input.ProductID, input.ProductUserID, input.OrganizationID, input.IncludePermissions,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Str("organization_id", input.OrganizationID).
			Err(err).
			Msg("failed to get user organization")
		return nil, err
	}

	return found.ToPtr(), nil
}

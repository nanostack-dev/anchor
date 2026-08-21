package service

import (
	"context"

	"anchor/internal/domain/organization"
	"anchor/internal/repository"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"
)

// newBodyProductUserNotFoundError answers a product_user_id supplied in a
// request body that does not resolve. Distinct from errors.go's
// NewProductUserNotFoundError (404), which is correct only where the caller
// names product_user_id in the path. The code differs too: one code at two
// statuses breaks a client that switches on code alone.
func newBodyProductUserNotFoundError(productUserID string) *fault.Error {
	return fault.BadRequest(
		"PRODUCT_USER_NOT_FOUND_IN_REQUEST",
		"This product has no product user with that identifier.",
	).Metadata(map[string]any{
		"product_user_id": productUserID,
	})
}

// newBodyRoleNotFoundError answers a role_id supplied in a request body that
// does not resolve. Distinct from errors.go's NewRoleNotFoundError (404),
// which stays correct for product_role_service.go, where role_id is a path
// parameter. The code differs too: one code at two statuses breaks a client
// that switches on code alone.
func newBodyRoleNotFoundError(roleID string) *fault.Error {
	return fault.BadRequest(
		"ROLE_NOT_FOUND_IN_REQUEST",
		"This product has no role with that identifier.",
	).Metadata(map[string]any{
		"role_id": roleID,
	})
}

// OrganizationMembershipService manages membership operations from the org perspective.
type OrganizationMembershipService interface {
	AddMember(ctx context.Context, input organization.AddMemberInput) (organization.Membership, error)
	UpdateMemberRole(ctx context.Context, input organization.UpdateMemberRoleInput) (organization.Membership, error)
	RemoveMember(ctx context.Context, input organization.RemoveMemberInput) error
	GetMember(ctx context.Context, input organization.GetMemberInput) (*organization.Membership, error)
	ListMembers(ctx context.Context, input organization.ListMembersInput) ([]organization.Membership, error)
	SearchMembers(
		ctx context.Context,
		input organization.SearchMembersInput,
	) (search.Result[organization.Membership], error)
}

type organizationMembershipService struct {
	orgMembershipRepo repository.OrganizationMembershipRepository
	productRoleRepo   repository.ProductRoleRepository
	productUserRepo   repository.ProductUserRepository
	organizationRepo  repository.OrganizationRepository
	logger            zerolog.Logger
}

func NewOrganizationMembershipService(
	orgMembershipRepo repository.OrganizationMembershipRepository,
	productRoleRepo repository.ProductRoleRepository,
	productUserRepo repository.ProductUserRepository,
	organizationRepo repository.OrganizationRepository,
	logger zerolog.Logger,
) OrganizationMembershipService {
	return &organizationMembershipService{
		orgMembershipRepo: orgMembershipRepo,
		productRoleRepo:   productRoleRepo,
		productUserRepo:   productUserRepo,
		organizationRepo:  organizationRepo,
		logger:            logger.With().Str("component", "organization_membership_service").Logger(),
	}
}

func (s *organizationMembershipService) AddMember(
	ctx context.Context, input organization.AddMemberInput,
) (organization.Membership, error) {
	logger := s.logger.With().Str("operation", "AddMember").Logger()

	if err := validateStruct(input); err != nil {
		return organization.Membership{}, err
	}

	if err := s.ensureOrganizationExists(ctx, input.ProductID, input.OrganizationID, logger); err != nil {
		return organization.Membership{}, err
	}

	if err := s.validateProductUser(ctx, input.ProductID, input.ProductUserID, logger); err != nil {
		return organization.Membership{}, err
	}

	if err := s.validateRole(ctx, input.ProductID, input.RoleID, logger); err != nil {
		return organization.Membership{}, err
	}

	if err := s.checkMembershipAbsence(
		ctx,
		input.ProductID,
		input.OrganizationID,
		input.ProductUserID,
		logger,
	); err != nil {
		return organization.Membership{}, err
	}

	return s.applyMembership(
		ctx,
		input.ProductID, input.OrganizationID, input.ProductUserID, input.RoleID,
		"failed to create membership", "member added to organization",
		s.orgMembershipRepo.Create,
		logger,
	)
}

func (s *organizationMembershipService) UpdateMemberRole(
	ctx context.Context, input organization.UpdateMemberRoleInput,
) (organization.Membership, error) {
	logger := s.logger.With().Str("operation", "UpdateMemberRole").Logger()

	if err := validateStruct(input); err != nil {
		return organization.Membership{}, err
	}

	if err := s.validateRole(ctx, input.ProductID, input.RoleID, logger); err != nil {
		return organization.Membership{}, err
	}

	if err := s.checkMembershipPresence(
		ctx,
		input.ProductID,
		input.OrganizationID,
		input.ProductUserID,
		logger,
	); err != nil {
		return organization.Membership{}, err
	}

	return s.applyMembership(
		ctx,
		input.ProductID, input.OrganizationID, input.ProductUserID, input.RoleID,
		"failed to update membership role", "member role updated",
		s.orgMembershipRepo.Update,
		logger,
	)
}

// checkMembershipAbsence verifies that no membership exists; returns an error if one does.
func (s *organizationMembershipService) checkMembershipAbsence(
	ctx context.Context,
	productID, organizationID, productUserID string,
	logger zerolog.Logger,
) error {
	found, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
		ctx, productID, organizationID, productUserID, false,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", productID).
			Str("organization_id", organizationID).
			Str("product_user_id", productUserID).
			Msg("failed to check existing membership")
		return err
	}
	if found.IsPresent() {
		return NewOrganizationMembershipAlreadyExistsError(productUserID, organizationID)
	}

	return nil
}

// checkMembershipPresence verifies that a membership exists; returns an error if it does not.
func (s *organizationMembershipService) checkMembershipPresence(
	ctx context.Context,
	productID, organizationID, productUserID string,
	logger zerolog.Logger,
) error {
	found, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
		ctx, productID, organizationID, productUserID, false,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", productID).
			Str("organization_id", organizationID).
			Str("product_user_id", productUserID).
			Msg("failed to verify existing membership")
		return err
	}
	if found.IsAbsent() {
		return NewOrganizationMembershipNotFoundError(productUserID, organizationID)
	}

	return nil
}

// applyMembership calls repoFn to persist a membership change, then returns the
// resulting domain object. It handles error logging and nil-result guarding so
// AddMember and UpdateMemberRole can share the common tail.
func (s *organizationMembershipService) applyMembership(
	ctx context.Context,
	productID, organizationID, productUserID, roleID string,
	failMsg, successMsg string,
	repoFn func(context.Context, string, string, string, string) (organization.Membership, error),
	logger zerolog.Logger,
) (organization.Membership, error) {
	membership, err := repoFn(ctx, productID, organizationID, productUserID, roleID)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", productID).
			Str("organization_id", organizationID).
			Str("product_user_id", productUserID).
			Str("role_id", roleID).
			Msg(failMsg)
		return organization.Membership{}, err
	}

	logger.Info().
		Str("product_id", productID).
		Str("organization_id", organizationID).
		Str("product_user_id", productUserID).
		Str("role_id", roleID).
		Msg(successMsg)

	return membership, nil
}

func (s *organizationMembershipService) RemoveMember(
	ctx context.Context, input organization.RemoveMemberInput,
) error {
	logger := s.logger.With().Str("operation", "RemoveMember").Logger()

	if err := validateStruct(input); err != nil {
		return err
	}

	found, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
		ctx, input.ProductID, input.OrganizationID, input.ProductUserID, false,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Str("product_user_id", input.ProductUserID).
			Msg("failed to verify membership before removal")
		return err
	}
	if found.IsAbsent() {
		return NewOrganizationMembershipNotFoundError(input.ProductUserID, input.OrganizationID)
	}

	if err = s.orgMembershipRepo.Delete(
		ctx, input.OrganizationID, input.ProductUserID,
	); err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Str("product_user_id", input.ProductUserID).
			Msg("failed to remove member from organization")
		return err
	}

	logger.Info().
		Str("product_id", input.ProductID).
		Str("organization_id", input.OrganizationID).
		Str("product_user_id", input.ProductUserID).
		Msg("member removed from organization")

	return nil
}

func (s *organizationMembershipService) GetMember(
	ctx context.Context, input organization.GetMemberInput,
) (*organization.Membership, error) {
	logger := s.logger.With().Str("operation", "GetMember").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	found, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
		ctx, input.ProductID, input.OrganizationID, input.ProductUserID, input.IncludePermissions,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Str("product_user_id", input.ProductUserID).
			Msg("failed to get member")
		return nil, err
	}

	return found.ToPtr(), nil
}

func (s *organizationMembershipService) ListMembers(
	ctx context.Context, input organization.ListMembersInput,
) ([]organization.Membership, error) {
	logger := s.logger.With().Str("operation", "ListMembers").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	memberships, err := s.orgMembershipRepo.FindByOrgID(
		ctx, input.ProductID, input.OrganizationID, input.IncludePermissions,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Msg("failed to list members")
		return nil, err
	}

	return memberships, nil
}

func (s *organizationMembershipService) SearchMembers(
	ctx context.Context, input organization.SearchMembersInput,
) (search.Result[organization.Membership], error) {
	logger := s.logger.With().Str("operation", "SearchMembers").Logger()

	if err := validateStruct(input); err != nil {
		return search.Result[organization.Membership]{}, err
	}

	result, err := s.orgMembershipRepo.SearchByOrgID(
		ctx, input.ProductID, input.OrganizationID, input.Request,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Msg("failed to search members")
		return search.Result[organization.Membership]{}, err
	}

	return result, nil
}

func (s *organizationMembershipService) validateProductUser(
	ctx context.Context,
	productID string,
	productUserID string,
	logger zerolog.Logger,
) error {
	found, err := s.productUserRepo.FindByProductIDAndID(ctx, productID, productUserID)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", productID).
			Str("product_user_id", productUserID).
			Msg("failed to verify product user exists")
		return err
	}
	if found.IsAbsent() {
		return newBodyProductUserNotFoundError(productUserID)
	}

	return nil
}

func (s *organizationMembershipService) validateRole(
	ctx context.Context,
	productID string,
	roleID string,
	logger zerolog.Logger,
) error {
	found, err := s.productRoleRepo.FindByProductIDAndRoleID(ctx, productID, roleID)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", productID).
			Str("role_id", roleID).
			Msg("failed to verify role exists")
		return err
	}
	if found.IsAbsent() {
		return newBodyRoleNotFoundError(roleID)
	}

	return nil
}

func (s *organizationMembershipService) ensureOrganizationExists(
	ctx context.Context,
	productID string,
	organizationID string,
	logger zerolog.Logger,
) error {
	found, err := s.organizationRepo.FindByID(ctx, productID, organizationID)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", productID).
			Str("organization_id", organizationID).
			Msg("failed to verify organization exists")
		return err
	}
	if found.IsAbsent() {
		return fault.ErrNotFound
	}

	return nil
}

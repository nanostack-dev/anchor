package service

import (
	"context"

	"anchor/internal/domain/organization"
	"anchor/internal/repository"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"
)

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
	organizationRepo  repository.OrganizationRepository
	productRoleRepo   repository.ProductRoleRepository
	productUserRepo   repository.ProductUserRepository
	logger            zerolog.Logger
}

func NewOrganizationMembershipService(
	orgMembershipRepo repository.OrganizationMembershipRepository,
	organizationRepo repository.OrganizationRepository,
	productRoleRepo repository.ProductRoleRepository,
	productUserRepo repository.ProductUserRepository,
	logger zerolog.Logger,
) OrganizationMembershipService {
	return &organizationMembershipService{
		orgMembershipRepo: orgMembershipRepo,
		organizationRepo:  organizationRepo,
		productRoleRepo:   productRoleRepo,
		productUserRepo:   productUserRepo,
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

	if err := s.validateOrganizationInProduct(ctx, input.ProductID, input.OrganizationID, logger); err != nil {
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

	if err := s.validateOrganizationInProduct(ctx, input.ProductID, input.OrganizationID, logger); err != nil {
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
	existing, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
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
	if existing != nil {
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
	existing, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
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
	if existing == nil {
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

	existing, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
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
	if existing == nil {
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

	membership, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
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

	return membership, nil
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

// validateOrganizationInProduct checks that the target organization exists and
// belongs to the caller's product. This is the tenant boundary: without it,
// AddMember/UpdateMemberRole are scoped only by the caller-supplied organization
// ID, so a caller authenticated for product A could write membership rows into
// an organization owned by product B (the membership row and its FK targets — a
// product-A user and product-A role — all satisfy the schema, and the follow-up
// membership reads scope by the *user's* product, not the organization's).
func (s *organizationMembershipService) validateOrganizationInProduct(
	ctx context.Context,
	productID string,
	organizationID string,
	logger zerolog.Logger,
) error {
	org, err := s.organizationRepo.FindByID(ctx, productID, organizationID)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", productID).
			Str("organization_id", organizationID).
			Msg("failed to verify organization belongs to product")
		return err
	}
	if org == nil {
		return NewOrganizationNotFoundError(organizationID)
	}

	return nil
}

// validateProductUser checks that the product user exists within the given product.
// The product itself is verified by the auth middleware; the organization is
// verified by validateOrganizationInProduct before this runs.
func (s *organizationMembershipService) validateProductUser(
	ctx context.Context,
	productID string,
	productUserID string,
	logger zerolog.Logger,
) error {
	user, err := s.productUserRepo.FindByProductIDAndID(ctx, productID, productUserID)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", productID).
			Str("product_user_id", productUserID).
			Msg("failed to verify product user exists")
		return err
	}
	if user == nil {
		return NewProductUserNotFoundError(productUserID)
	}

	return nil
}

func (s *organizationMembershipService) validateRole(
	ctx context.Context,
	productID string,
	roleID string,
	logger zerolog.Logger,
) error {
	role, err := s.productRoleRepo.FindByProductIDAndRoleID(ctx, productID, roleID)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", productID).
			Str("role_id", roleID).
			Msg("failed to verify role exists")
		return err
	}
	if role == nil {
		return NewRoleNotFoundError(roleID)
	}

	return nil
}

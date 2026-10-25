package service

import (
	"context"
	"errors"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/organizationrole"
	"anchor/internal/events"
	orgroleservice "anchor/internal/organizationrole/service"
	"anchor/internal/repository"
	"anchor/internal/searchx"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"
)

// NewBodyProductUserNotFoundError answers a product_user_id supplied in a
// request body that does not resolve. Distinct from user.ErrProductUserNotFound
// (404), which is correct only where the caller names product_user_id in the
// path. The code differs too: one code at two
// statuses breaks a client that switches on code alone.
func NewBodyProductUserNotFoundError(productUserID string) *fault.Error {
	return fault.BadRequest(
		"PRODUCT_USER_NOT_FOUND_IN_REQUEST",
		"This product has no product user with that identifier.",
	).Metadata(map[string]any{
		"product_user_id": productUserID,
	})
}

// NewBodyRoleNotFoundError answers a role_id supplied in a request body that
// does not resolve. Distinct from role.ErrProductRoleNotFound (404), which
// stays correct for product_role_service.go, where role_id is a path
// parameter.
func NewBodyRoleNotFoundError(roleID string) *fault.Error {
	return organizationrole.NewBodyRoleNotFoundError(roleID)
}

// OrganizationMembershipService manages membership operations from the org perspective.
type OrganizationMembershipService interface {
	AddMember(ctx context.Context, input organization.AddMemberInput) (organization.Membership, error)
	UpdateMemberRole(ctx context.Context, input organization.UpdateMemberRoleInput) (organization.Membership, error)
	RemoveMember(ctx context.Context, input organization.RemoveMemberInput) error
	GetMember(ctx context.Context, input organization.GetMemberInput) (organization.Membership, error)
	ListMembers(ctx context.Context, input organization.ListMembersInput) ([]organization.Membership, error)
	SearchMembers(
		ctx context.Context,
		input organization.SearchMembersInput,
	) (search.Result[organization.Membership], error)
}

type organizationMembershipService struct {
	orgMembershipRepo repository.OrganizationMembershipRepository
	organizationRoles orgroleservice.Service
	productUserRepo   repository.ProductUserRepository
	organizationRepo  repository.OrganizationRepository
	transactor        transactor.Transactor
	events            events.Emitter
	logger            zerolog.Logger
}

func NewOrganizationMembershipService(
	orgMembershipRepo repository.OrganizationMembershipRepository,
	organizationRoles orgroleservice.Service,
	productUserRepo repository.ProductUserRepository,
	organizationRepo repository.OrganizationRepository,
	tx transactor.Transactor,
	eventEmitter events.Emitter,
	logger zerolog.Logger,
) OrganizationMembershipService {
	return &organizationMembershipService{
		orgMembershipRepo: orgMembershipRepo,
		organizationRoles: organizationRoles,
		productUserRepo:   productUserRepo,
		organizationRepo:  organizationRepo,
		transactor:        tx,
		events:            eventEmitter,
		logger:            logger.With().Str("component", "organization_membership_service").Logger(),
	}
}

func (s *organizationMembershipService) emitMembership(
	ctx context.Context, eventType events.Type, productID, organizationID, productUserID string,
) error {
	return s.events.Emit(ctx, events.Event{
		Type:      eventType,
		ProductID: productID,
		Data: events.Data{
			events.FieldOrganizationID: organizationID,
			events.FieldProductUserID:  productUserID,
		},
	})
}

func (s *organizationMembershipService) AddMember(
	ctx context.Context, input organization.AddMemberInput,
) (organization.Membership, error) {
	logger := s.logger.With().Str("operation", "AddMember").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return organization.Membership{}, err
	}

	if err := s.ensureOrganizationExists(ctx, input.ProductID, input.OrganizationID, logger); err != nil {
		return organization.Membership{}, err
	}

	if err := s.validateProductUser(ctx, input.ProductID, input.ProductUserID, logger); err != nil {
		return organization.Membership{}, err
	}

	if err := s.validateRole(ctx, input.ProductID, input.OrganizationID, input.RoleID); err != nil {
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
		events.MembershipCreated,
		"failed to create membership", "member added to organization",
		s.orgMembershipRepo.Create,
		logger,
	)
}

func (s *organizationMembershipService) UpdateMemberRole(
	ctx context.Context, input organization.UpdateMemberRoleInput,
) (organization.Membership, error) {
	logger := s.logger.With().Str("operation", "UpdateMemberRole").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return organization.Membership{}, err
	}

	if err := s.validateRole(ctx, input.ProductID, input.OrganizationID, input.RoleID); err != nil {
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
		events.MembershipUpdated,
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
		ctx, productID, organizationID, productUserID, organization.WithoutRolePermissions,
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
		ctx, productID, organizationID, productUserID, organization.WithoutRolePermissions,
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
		return organization.ErrMembershipNotFound
	}

	return nil
}

// applyMembership calls repoFn to persist a membership change, then returns the
// resulting domain object. It handles error logging and nil-result guarding so
// AddMember and UpdateMemberRole can share the common tail.
func (s *organizationMembershipService) applyMembership(
	ctx context.Context,
	productID, organizationID, productUserID, roleID string,
	eventType events.Type,
	failMsg, successMsg string,
	repoFn func(context.Context, string, string, string, string) (organization.Membership, error),
	logger zerolog.Logger,
) (organization.Membership, error) {
	var membership organization.Membership
	if txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		role, err := s.resolveRole(txCtx, productID, organizationID, roleID)
		if err != nil {
			return err
		}
		membership, err = repoFn(txCtx, productID, organizationID, productUserID, role.RowID)
		if err != nil {
			return err
		}
		return s.emitMembership(txCtx, eventType, productID, organizationID, productUserID)
	}); txErr != nil {
		if errors.Is(txErr, repository.ErrOrganizationMembershipExists) {
			return organization.Membership{}, NewOrganizationMembershipAlreadyExistsError(productUserID, organizationID)
		}
		if _, isFault := fault.As(txErr); isFault {
			return organization.Membership{}, txErr
		}
		logger.Error().Err(txErr).
			Str("product_id", productID).
			Str("organization_id", organizationID).
			Str("product_user_id", productUserID).
			Str("role_id", roleID).
			Msg(failMsg)
		return organization.Membership{}, txErr
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

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}

	found, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
		ctx, input.ProductID, input.OrganizationID, input.ProductUserID, organization.WithoutRolePermissions,
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
		return organization.ErrMembershipNotFound
	}

	if txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if delErr := s.orgMembershipRepo.Delete(txCtx, input.OrganizationID, input.ProductUserID); delErr != nil {
			return delErr
		}
		return s.emitMembership(
			txCtx, events.MembershipDeleted, input.ProductID, input.OrganizationID, input.ProductUserID,
		)
	}); txErr != nil {
		logger.Error().Err(txErr).
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Str("product_user_id", input.ProductUserID).
			Msg("failed to remove member from organization")
		return txErr
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
) (organization.Membership, error) {
	logger := s.logger.With().Str("operation", "GetMember").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return organization.Membership{}, err
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
		return organization.Membership{}, err
	}

	return found.ToResult(organization.ErrMembershipNotFound).Value()
}

func (s *organizationMembershipService) ListMembers(
	ctx context.Context, input organization.ListMembersInput,
) ([]organization.Membership, error) {
	logger := s.logger.With().Str("operation", "ListMembers").Logger()

	if err := validate.ValidateStruct(input); err != nil {
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

	if err := input.Validate(); err != nil {
		return search.Result[organization.Membership]{}, err
	}

	result, err := s.orgMembershipRepo.SearchByOrgID(
		ctx, input.ProductID, input.OrganizationID, input.Request,
	)
	if err != nil {
		if !searchx.IsClientError(err) {
			logger.Error().Err(err).
				Str("product_id", input.ProductID).
				Str("organization_id", input.OrganizationID).
				Msg("failed to search members")
		}
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
		return NewBodyProductUserNotFoundError(productUserID)
	}

	return nil
}

// validateRole answers a role_id that names no role of the Organization and
// no role template of its Product before any membership check, as it always
// has.
func (s *organizationMembershipService) validateRole(
	ctx context.Context, productID, organizationID, roleID string,
) error {
	_, err := s.organizationRoles.Resolve(ctx, productID, organizationID, roleID)
	return err
}

// resolveRole finds the Organization's role the request named: a template's
// ID names its copy when the Organization has one.
func (s *organizationMembershipService) resolveRole(
	txCtx context.Context, productID, organizationID, roleID string,
) (organizationrole.Role, error) {
	return s.organizationRoles.Resolve(txCtx, productID, organizationID, roleID)
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
		return organization.ErrOrganizationNotFound
	}

	return nil
}

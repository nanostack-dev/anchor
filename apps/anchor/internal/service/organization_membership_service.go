package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/product/role"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/events"
	"anchor/internal/repository"
	rtservice "anchor/internal/resourcetype/service"
	assignments "anchor/internal/roleassignment/service"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
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

// NewBodyRoleNotOrganizationScopedError refuses a role scoped below
// organization as an Organization role: its permissions sit at or below its
// scope, so it would grant nothing on the Organization (ADR-0022 D4).
func NewBodyRoleNotOrganizationScopedError(roleID, resourceType string) *fault.Error {
	return fault.BadRequest(
		"ROLE_NOT_ORGANIZATION_SCOPED",
		fmt.Sprintf(
			"An Organization role must be scoped to organization. This role is scoped to %s: assign it on a resource instead.",
			resourceType,
		),
	).Metadata(map[string]any{fieldRoleIDKey: roleID, "resource_type": resourceType})
}

// NewBodyRoleNotFoundError answers a role_id supplied in a request body that
// does not resolve. Distinct from role.ErrProductRoleNotFound (404), which
// stays correct for product_role_service.go, where role_id is a path
// parameter. The code differs too: one code at two statuses breaks a client
// that switches on code alone.
func NewBodyRoleNotFoundError(roleID string) *fault.Error {
	return fault.BadRequest(
		"ROLE_NOT_FOUND_IN_REQUEST",
		"This product has no role with that identifier.",
	).Metadata(map[string]any{
		fieldRoleIDKey: roleID,
	})
}

// OrganizationMembershipService manages membership operations from the org perspective.
type OrganizationMembershipService interface {
	AddMember(ctx context.Context, input organization.AddMemberInput) (organization.Membership, error)
	UpdateMemberRole(ctx context.Context, input organization.UpdateMemberRoleInput) (organization.Membership, error)
	RemoveMember(ctx context.Context, input organization.RemoveMemberInput) error
	GetMember(ctx context.Context, input organization.GetMemberInput) (organization.Membership, error)
	ListMembers(ctx context.Context, input organization.ListMembersInput) ([]organization.Membership, error)
	SearchMembers(
		ctx context.Context, q repository.OrganizationMemberSearch,
	) (search.Result[organization.Membership], error)
}

type organizationMembershipService struct {
	orgMembershipRepo repository.OrganizationMembershipRepository
	productRoleRepo   repository.ProductRoleRepository
	resourceTypes     rtservice.Service
	roleAssignments   assignments.Service
	productUserRepo   repository.ProductUserRepository
	organizationRepo  repository.OrganizationRepository
	transactor        transactor.Transactor
	events            events.Emitter
	logger            *slog.Logger
}

func NewOrganizationMembershipService(
	orgMembershipRepo repository.OrganizationMembershipRepository,
	productRoleRepo repository.ProductRoleRepository,
	resourceTypes rtservice.Service,
	roleAssignments assignments.Service,
	productUserRepo repository.ProductUserRepository,
	organizationRepo repository.OrganizationRepository,
	tx transactor.Transactor,
	eventEmitter events.Emitter,
	logger *slog.Logger,
) OrganizationMembershipService {
	return &organizationMembershipService{
		orgMembershipRepo: orgMembershipRepo,
		productRoleRepo:   productRoleRepo,
		resourceTypes:     resourceTypes,
		roleAssignments:   roleAssignments,
		productUserRepo:   productUserRepo,
		organizationRepo:  organizationRepo,
		transactor:        tx,
		events:            eventEmitter,
		logger:            logger.With(slog.String("component", "organization_membership_service")),
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
	logger := s.logger.With(slog.String("operation", "AddMember"))

	if err := validate.ValidateStruct(input); err != nil {
		return organization.Membership{}, err
	}

	if err := s.ensureOrganizationExists(ctx, input.ProductID, input.OrganizationID, logger); err != nil {
		return organization.Membership{}, err
	}

	if err := s.validateProductUser(ctx, input.ProductID, input.ProductUserID, logger); err != nil {
		return organization.Membership{}, err
	}

	if err := s.validateRole(ctx, input.ProductID, input.OrganizationID, input.RoleID, logger); err != nil {
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
	logger := s.logger.With(slog.String("operation", "UpdateMemberRole"))

	if err := validate.ValidateStruct(input); err != nil {
		return organization.Membership{}, err
	}

	if err := s.validateRole(ctx, input.ProductID, input.OrganizationID, input.RoleID, logger); err != nil {
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
	logger *slog.Logger,
) error {
	found, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
		ctx, productID, organizationID, productUserID, organization.WithoutRolePermissions,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to check existing membership",
			log.Err(err),
			slog.String("product_id", productID),
			slog.String("organization_id", organizationID),
			slog.String("product_user_id", productUserID),
		)
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
	logger *slog.Logger,
) error {
	found, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
		ctx, productID, organizationID, productUserID, organization.WithoutRolePermissions,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to verify existing membership",
			log.Err(err),
			slog.String("product_id", productID),
			slog.String("organization_id", organizationID),
			slog.String("product_user_id", productUserID),
		)
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
	productID, organizationID, productUserID string, roleID *string,
	eventType events.Type,
	failMsg, successMsg string,
	repoFn func(context.Context, string, string, string, *string) (organization.Membership, error),
	logger *slog.Logger,
) (organization.Membership, error) {
	var membership organization.Membership
	if txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		var err error
		membership, err = repoFn(txCtx, productID, organizationID, productUserID, roleID)
		if err != nil {
			return err
		}
		return s.emitMembership(txCtx, eventType, productID, organizationID, productUserID)
	}); txErr != nil {
		if errors.Is(txErr, repository.ErrOrganizationMembershipExists) {
			return organization.Membership{}, NewOrganizationMembershipAlreadyExistsError(productUserID, organizationID)
		}
		if errors.Is(txErr, repository.ErrOrganizationMembershipRoleGone) {
			return organization.Membership{}, NewBodyRoleNotFoundError(loggedRoleID(roleID))
		}
		log.ErrorTo(ctx, logger, txErr, failMsg,
			slog.String("product_id", productID),
			slog.String("organization_id", organizationID),
			slog.String("product_user_id", productUserID),
			slog.String("role_id", loggedRoleID(roleID)),
		)
		return organization.Membership{}, txErr
	}

	logger.InfoContext(ctx, successMsg,
		slog.String("product_id", productID),
		slog.String("organization_id", organizationID),
		slog.String("product_user_id", productUserID),
		slog.String("role_id", loggedRoleID(roleID)),
	)

	return membership, nil
}

func (s *organizationMembershipService) RemoveMember(
	ctx context.Context, input organization.RemoveMemberInput,
) error {
	logger := s.logger.With(slog.String("operation", "RemoveMember"))

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}

	found, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
		ctx, input.ProductID, input.OrganizationID, input.ProductUserID, organization.WithoutRolePermissions,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to verify membership before removal",
			log.Err(err),
			slog.String("product_id", input.ProductID),
			slog.String("organization_id", input.OrganizationID),
			slog.String("product_user_id", input.ProductUserID),
		)
		return err
	}
	if found.IsAbsent() {
		return organization.ErrMembershipNotFound
	}

	if txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		// The member's role assignments on resources go with the membership
		// through its foreign key, each with its event.
		if releaseErr := s.roleAssignments.ReleaseMember(
			txCtx, input.ProductID, input.OrganizationID, input.ProductUserID,
		); releaseErr != nil {
			return releaseErr
		}
		if delErr := s.orgMembershipRepo.DeleteByOrganizationIDAndProductUserID(
			txCtx, input.OrganizationID, input.ProductUserID,
		); delErr != nil {
			return delErr
		}
		return s.emitMembership(
			txCtx, events.MembershipDeleted, input.ProductID, input.OrganizationID, input.ProductUserID,
		)
	}); txErr != nil {
		logger.ErrorContext(ctx, "failed to remove member from organization",
			log.Err(txErr),
			slog.String("product_id", input.ProductID),
			slog.String("organization_id", input.OrganizationID),
			slog.String("product_user_id", input.ProductUserID),
		)
		return txErr
	}

	logger.InfoContext(ctx, "member removed from organization",
		slog.String("product_id", input.ProductID),
		slog.String("organization_id", input.OrganizationID),
		slog.String("product_user_id", input.ProductUserID),
	)

	return nil
}

func (s *organizationMembershipService) GetMember(
	ctx context.Context, input organization.GetMemberInput,
) (organization.Membership, error) {
	logger := s.logger.With(slog.String("operation", "GetMember"))

	if err := validate.ValidateStruct(input); err != nil {
		return organization.Membership{}, err
	}

	found, err := s.orgMembershipRepo.FindByOrgIDAndUserID(
		ctx, input.ProductID, input.OrganizationID, input.ProductUserID, input.IncludePermissions,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get member",
			log.Err(err),
			slog.String("product_id", input.ProductID),
			slog.String("organization_id", input.OrganizationID),
			slog.String("product_user_id", input.ProductUserID),
		)
		return organization.Membership{}, err
	}

	return found.ToResult(organization.ErrMembershipNotFound).Value()
}

func (s *organizationMembershipService) ListMembers(
	ctx context.Context, input organization.ListMembersInput,
) ([]organization.Membership, error) {
	logger := s.logger.With(slog.String("operation", "ListMembers"))

	if err := validate.ValidateStruct(input); err != nil {
		return nil, err
	}

	memberships, err := s.orgMembershipRepo.FindByOrgID(
		ctx, input.ProductID, input.OrganizationID, input.IncludePermissions,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to list members",
			log.Err(err),
			slog.String("product_id", input.ProductID),
			slog.String("organization_id", input.OrganizationID),
		)
		return nil, err
	}

	return memberships, nil
}

func (s *organizationMembershipService) SearchMembers(
	ctx context.Context, q repository.OrganizationMemberSearch,
) (search.Result[organization.Membership], error) {
	logger := s.logger.With(slog.String("operation", "SearchMembers"))

	result, err := s.orgMembershipRepo.SearchByOrgID(ctx, q)
	if err != nil {
		if !search.IsClientError(err) {
			logger.ErrorContext(ctx, "failed to search members",
				log.Err(err),
				slog.String("product_id", q.Scope().ProductID),
				slog.String("organization_id", q.Scope().OrganizationID),
			)
		}
		return search.Result[organization.Membership]{}, err
	}

	return result, nil
}

func (s *organizationMembershipService) validateProductUser(
	ctx context.Context,
	productID string,
	productUserID string,
	logger *slog.Logger,
) error {
	found, err := s.productUserRepo.FindByProductIDAndID(ctx, productID, productUserID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to verify product user exists",
			log.Err(err),
			slog.String("product_id", productID),
			slog.String("product_user_id", productUserID),
		)
		return err
	}
	if found.IsAbsent() {
		return NewBodyProductUserNotFoundError(productUserID)
	}

	return nil
}

func (s *organizationMembershipService) validateRole(
	ctx context.Context,
	productID, organizationID string, roleID *string,
	logger *slog.Logger,
) error {
	err := CheckAssignableRole(ctx, s.productRoleRepo, s.resourceTypes, productID, organizationID, roleID)
	if _, isFault := fault.As(err); err != nil && !isFault {
		logger.ErrorContext(ctx, "failed to verify role exists",
			log.Err(err),
			slog.String("product_id", productID),
			slog.String("role_id", loggedRoleID(roleID)),
		)
	}
	return err
}

// CheckAssignableRole answers a role_id in a request body that a member of the
// Organization may not hold. A member may hold a role the Product manages or
// a custom role of their own Organization; another Organization's role answers
// as an unknown one, so a request learns nothing about it. The role must also
// be scoped to organization (ADR-0022 D4). No role (nil) is always allowed:
// the member holds no permission.
func CheckAssignableRole(
	ctx context.Context, roles repository.ProductRoleRepository, resourceTypes rtservice.Service,
	productID, organizationID string, roleID *string,
) error {
	if roleID == nil {
		return nil
	}
	found, err := roles.FindInProductByID(ctx, productID, *roleID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return NewBodyRoleNotFoundError(*roleID)
	}
	named := found.Value()
	if !named.AssignableIn(organizationID) {
		return NewBodyRoleNotFoundError(*roleID)
	}
	return CheckOrganizationScoped(ctx, resourceTypes, named)
}

// CheckOrganizationScoped refuses a role scoped to any type but organization
// as an Organization role.
func CheckOrganizationScoped(ctx context.Context, resourceTypes rtservice.Service, r role.ProductRole) error {
	tree, err := resourceTypes.Tree(ctx, r.ProductID)
	if err != nil {
		return err
	}
	scope := tree.ByID(r.ResourceTypeID).Map(func(rt resourcetype.ResourceType) string { return rt.Slug }).
		OrElse(resourcetype.OrganizationSlug)
	if scope != resourcetype.OrganizationSlug {
		return NewBodyRoleNotOrganizationScopedError(r.ID, scope)
	}
	return nil
}

// loggedRoleID is a role_id for a log line or an error: empty for no role.
func loggedRoleID(roleID *string) string {
	return functional.FromPtr(roleID).OrElse("")
}

func (s *organizationMembershipService) ensureOrganizationExists(
	ctx context.Context,
	productID string,
	organizationID string,
	logger *slog.Logger,
) error {
	found, err := s.organizationRepo.FindByProductIDAndID(ctx, productID, organizationID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to verify organization exists",
			log.Err(err),
			slog.String("product_id", productID),
			slog.String("organization_id", organizationID),
		)
		return err
	}
	if found.IsAbsent() {
		return organization.ErrOrganizationNotFound
	}

	return nil
}

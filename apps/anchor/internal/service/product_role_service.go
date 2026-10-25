package service

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	"anchor/internal/domain/organizationrole"
	resourcepermission "anchor/internal/domain/product/resource_permission"
	role "anchor/internal/domain/product/role"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/events"
	invitationrepository "anchor/internal/invitation/repository"
	orgroleservice "anchor/internal/organizationrole/service"
	"anchor/internal/repository"
	rtservice "anchor/internal/resourcetype/service"

	"github.com/rs/zerolog"
)

// NewProductRoleResourcePermissionNotFoundError reports a resource
// permission named by its path segment (unassignPermissionFromProductRole's
// permission_id) that does not exist. Distinct from
// service.NewPermissionsNotFoundError's PERMISSIONS_NOT_FOUND, which answers
// a body-supplied list of permission names as a 400: this identifier is
// path-addressed, so it is a 404.
func NewProductRoleResourcePermissionNotFoundError(productID, permissionName string) *fault.Error {
	return resourcepermission.ErrResourcePermissionNotFound.
		Metadata(map[string]any{
			"product_id":      productID,
			"permission_name": permissionName,
		})
}

type ProductRoleService interface {
	CreateProductRole(
		ctx context.Context, input role.CreateProductRoleInput,
	) (role.ProductRole, error)
	SearchProductRoles(
		ctx context.Context, input role.SearchProductRolesInput,
	) (search.Result[role.ProductRole], error)
	GetProductRole(
		ctx context.Context, input role.GetProductRoleInput,
	) (role.ProductRole, error)
	UpdateProductRole(
		ctx context.Context, input role.UpdateProductRoleInput,
	) (role.ProductRole, error)
	DeleteProductRole(
		ctx context.Context, input role.DeleteProductRoleInput,
	) error
	AssignPermissionToProductRole(
		ctx context.Context, input role.AssignPermissionToProductRoleInput,
	) error
	UnassignPermissionFromProductRole(
		ctx context.Context, input role.UnassignPermissionFromProductRoleInput,
	) error
}

type productRoleService struct {
	roleRepo                      repository.ProductRoleRepository
	productResourcePermissionRepo repository.ProductResourcePermissionRepository
	invitationRepo                invitationrepository.Repository
	resourceTypes                 rtservice.Service
	organizationRoles             orgroleservice.Service
	transactor                    transactor.Transactor
	events                        events.Emitter
	logger                        zerolog.Logger
}

func NewProductRoleService(
	roleRepo repository.ProductRoleRepository,
	productResourcePermissionRepo repository.ProductResourcePermissionRepository,
	invitationRepo invitationrepository.Repository,
	resourceTypes rtservice.Service,
	organizationRoles orgroleservice.Service,
	tx transactor.Transactor,
	eventEmitter events.Emitter,
	logger zerolog.Logger,
) ProductRoleService {
	return &productRoleService{
		roleRepo:                      roleRepo,
		productResourcePermissionRepo: productResourcePermissionRepo,
		invitationRepo:                invitationRepo,
		resourceTypes:                 resourceTypes,
		organizationRoles:             organizationRoles,
		transactor:                    tx,
		events:                        eventEmitter,
		logger: logger.With().Str(
			"component", "product_role_permission_service",
		).Logger(),
	}
}

func (s *productRoleService) emitRole(
	ctx context.Context, eventType events.Type, productID, roleID string,
) error {
	return s.events.Emit(ctx, events.Event{
		Type:      eventType,
		ProductID: productID,
		Data:      events.Data{events.FieldRoleID: roleID},
	})
}

func (s *productRoleService) CreateProductRole(
	ctx context.Context, input role.CreateProductRoleInput,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "CreateProductRole").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return role.ProductRole{}, err
	}
	if input.ResourceType == "" {
		return role.ProductRole{}, resourcetype.ErrRequired
	}
	if err := s.nameDuplicationValidation(ctx, input.ProductID, input.Name, "", logger); err != nil {
		return role.ProductRole{}, err
	}
	productRole := role.ProductRole{
		ProductID:   input.ProductID,
		Name:        input.Name,
		Description: input.Description,
		Key:         cmp.Or(input.Key, strings.ToLower(input.Name)),
		IsDefault:   input.IsDefault,
	}
	if err := s.keyDuplicationValidation(ctx, input.ProductID, input.Name, productRole.Key, logger); err != nil {
		return role.ProductRole{}, err
	}
	productRole.GenerateID()
	logger.Info().
		Str("product_role_id", productRole.ID).
		Str("product_id", input.ProductID).
		Str("name", input.Name).
		Msg("creating product role")

	productRolePermission := functional.Slice(
		input.Permissions).Map(
		func(perm string) role.ProductRolePermission {
			p := role.ProductRolePermission{
				ProductRoleID:  productRole.ID,
				ProductID:      input.ProductID,
				PermissionName: perm,
			}
			p.GenerateID()
			return p
		})

	resolvedPermissions, err := s.resolveRolePermissions(ctx, input.ProductID, productRolePermission, logger)
	if err != nil {
		return role.ProductRole{}, err
	}

	productRole.Permissions = resolvedPermissions

	var created role.ProductRole
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		tree, treeErr := s.resourceTypes.LockedTree(txCtx, input.ProductID)
		if treeErr != nil {
			return treeErr
		}
		scope, known := tree.BySlug(input.ResourceType).Get()
		if !known {
			return resourcetype.NewUnknownError([]string{input.ResourceType})
		}
		if scopeErr := s.checkBundle(
			txCtx,
			tree,
			input.ProductID,
			scope.Slug,
			productRole.Permissions,
		); scopeErr != nil {
			return scopeErr
		}
		productRole.ResourceTypeID = &scope.ID
		if keyErr := s.claimTemplateKey(txCtx, input.ProductID, productRole.Key); keyErr != nil {
			return keyErr
		}
		var cleared []string
		if productRole.IsDefault {
			var defaultErr error
			if cleared, defaultErr = s.takeDefault(txCtx, input.ProductID, "", scope.Slug); defaultErr != nil {
				return defaultErr
			}
		}

		var createErr error
		created, createErr = s.roleRepo.Create(txCtx, productRole)
		if errors.Is(createErr, repository.ErrProductRoleKeyExists) {
			return createErr
		}
		if mapped := mapTemplateWriteError(createErr, input.Name, productRole.Key, input.ProductID); mapped != nil {
			return mapped
		}
		if createErr != nil {
			logger.Error().
				Str("product_id", input.ProductID).
				Str("name", input.Name).
				Err(createErr).
				Msg("failed to create product role")
			return fault.ErrUnexpected
		}
		created.ResourceType = scope.Slug
		if emitErr := s.emitRoles(txCtx, input.ProductID, cleared); emitErr != nil {
			return emitErr
		}
		return s.emitRole(txCtx, events.ProductRoleCreated, input.ProductID, created.ID)
	})
	if errors.Is(err, repository.ErrProductRoleKeyExists) {
		return role.ProductRole{}, s.keyConflict(ctx, input.ProductID, input.Name, productRole.Key, logger)
	}
	if err != nil {
		return role.ProductRole{}, err
	}

	logger.Info().
		Str("product_role_id", created.ID).
		Str("product_id", input.ProductID).
		Str("name", input.Name).
		Msg("product role created")

	return created, nil
}

func (s *productRoleService) SearchProductRoles(
	ctx context.Context, input role.SearchProductRolesInput,
) (search.Result[role.ProductRole], error) {
	logger := s.logger.With().Str("operation", "SearchProductRoles").Logger()

	if err := input.Validate(); err != nil {
		return search.Result[role.ProductRole]{}, err
	}

	return scopedSearch(ctx, s.resourceTypes, logger, input.ProductID, input.Request,
		role.SearchProductRoleFilter.WithScopes, s.roleRepo.SearchByProductID, withRoleScope)
}

func (s *productRoleService) GetProductRole(
	ctx context.Context, input role.GetProductRoleInput,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "GetProductRole").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return role.ProductRole{}, err
	}

	found, err := s.roleRepo.FindByProductIDAndRoleID(ctx, input.ProductID, input.ID)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to get product role")
		return role.ProductRole{}, fault.ErrUnexpected
	}
	if found.IsAbsent() {
		return role.ProductRole{}, role.ErrProductRoleNotFound
	}
	return s.withScope(ctx, input.ProductID, found.Value(), logger)
}

func (s *productRoleService) UpdateProductRole(
	ctx context.Context, input role.UpdateProductRoleInput,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "UpdateProductRole").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return role.ProductRole{}, err
	}

	logger.Info().
		Str("product_role_id", input.ID).
		Str("product_id", input.ProductID).
		Msg("updating product role")

	foundRole, err := s.roleRepo.FindByProductIDAndRoleID(ctx, input.ProductID, input.ID)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to find product role for update")
		return role.ProductRole{}, fault.ErrUnexpected
	}
	if foundRole.IsAbsent() {
		return role.ProductRole{}, role.ErrProductRoleNotFound
	}

	updatedRole, err := s.withScope(ctx, input.ProductID, foundRole.Value(), logger)
	if err != nil {
		return role.ProductRole{}, err
	}
	if input.ResourceType != nil && *input.ResourceType != updatedRole.ResourceType {
		return role.ProductRole{}, role.ErrScopeImmutable
	}

	if input.Name != nil && *input.Name != updatedRole.Name {
		err = s.nameDuplicationValidation(ctx, input.ProductID, *input.Name, input.ID, logger)
		if err != nil {
			return role.ProductRole{}, err
		}
		updatedRole.Name = *input.Name
	}
	if input.Description != nil {
		updatedRole.Description = *input.Description
	}
	if input.Permissions != nil {
		input.Permissions, err = s.resolveRolePermissions(ctx, input.ProductID, input.Permissions, logger)
		if err != nil {
			return role.ProductRole{}, err
		}
		for i := range input.Permissions {
			input.Permissions[i].GenerateID()
		}
	}

	var updated role.ProductRole
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		// The catalog lock comes before the template lock, in the order every
		// role write takes them.
		var tree resourcetype.Tree
		if input.Permissions != nil {
			var treeErr error
			if tree, treeErr = s.resourceTypes.LockedTree(txCtx, input.ProductID); treeErr != nil {
				return treeErr
			}
		}
		cleared, defaultErr := s.applyTemplateDefault(txCtx, input, updatedRole.ResourceType)
		if defaultErr != nil {
			return defaultErr
		}
		var updateErr error
		updated, updateErr = s.roleRepo.Update(txCtx, updatedRole)
		if mapped := mapTemplateWriteError(
			updateErr,
			updatedRole.Name,
			updatedRole.Key,
			input.ProductID,
		); mapped != nil {
			return mapped
		}
		if updateErr != nil {
			logger.Error().
				Str("product_id", input.ProductID).
				Str("role_id", input.ID).
				Err(updateErr).
				Msg("failed to update product role")
			return fault.ErrUnexpected
		}
		if input.Permissions != nil {
			if scopeErr := s.checkBundle(
				txCtx,
				tree,
				input.ProductID,
				updatedRole.ResourceType,
				input.Permissions,
			); scopeErr != nil {
				return scopeErr
			}
			if updated, updateErr = s.replacePermissions(txCtx, input, logger); updateErr != nil {
				return updateErr
			}
		}
		updated.ResourceType = updatedRole.ResourceType
		if syncErr := s.organizationRoles.SyncTemplate(txCtx, input.ProductID, input.ID); syncErr != nil {
			return syncErr
		}
		if emitErr := s.emitRoles(txCtx, input.ProductID, cleared); emitErr != nil {
			return emitErr
		}
		return s.emitRole(txCtx, events.ProductRoleUpdated, input.ProductID, updated.ID)
	})
	if err != nil {
		return role.ProductRole{}, err
	}

	logger.Info().
		Str("product_role_id", input.ID).
		Str("product_id", input.ProductID).
		Msg("product role updated")

	return updated, nil
}

func (s *productRoleService) replacePermissions(
	ctx context.Context, input role.UpdateProductRoleInput, logger zerolog.Logger,
) (role.ProductRole, error) {
	if err := s.roleRepo.ReplacePermissions(ctx, input.ProductID, input.ID, input.Permissions); err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to replace product role permissions")
		return role.ProductRole{}, fault.ErrUnexpected
	}
	found, err := s.roleRepo.FindByProductIDAndRoleID(ctx, input.ProductID, input.ID)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to reload product role after replacing permissions")
		return role.ProductRole{}, fault.ErrUnexpected
	}
	return found.ToResult(role.ErrProductRoleNotFound).Value()
}

func (s *productRoleService) DeleteProductRole(
	ctx context.Context, input role.DeleteProductRoleInput,
) error {
	logger := s.logger.With().Str("operation", "DeleteProductRole").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}

	foundRole, err := s.roleRepo.FindByProductIDAndRoleID(ctx, input.ProductID, input.ID)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to find product role for deletion")
		return fault.ErrUnexpected
	}
	if foundRole.IsAbsent() {
		return role.ErrProductRoleNotFound
	}

	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if lockErr := s.roleRepo.LockTemplate(txCtx, input.ProductID, input.ID); lockErr != nil {
			return lockErr
		}
		if detachErr := s.detachWhenCustomRolesOn(txCtx, input.ProductID, input.ID); detachErr != nil {
			return detachErr
		}
		assignmentCount, countErr := s.roleRepo.CountMembershipAssignments(txCtx, input.ID)
		if countErr != nil {
			logger.Error().
				Str("product_id", input.ProductID).
				Str("role_id", input.ID).
				Err(countErr).
				Msg("failed to count role membership assignments")
			return fault.ErrUnexpected
		}
		if assignmentCount > 0 {
			return NewRoleInUseError(input.ID)
		}
		if invitationErr := s.releaseInvitations(txCtx, input.ProductID, input.ID, logger); invitationErr != nil {
			return invitationErr
		}
		if deleteErr := s.roleRepo.DeleteByProductIDAndRoleID(
			txCtx, input.ProductID, input.ID,
		); deleteErr != nil {
			logger.Error().
				Str("product_id", input.ProductID).
				Str("role_id", input.ID).
				Err(deleteErr).
				Msg("failed to delete product role")
			return fault.ErrUnexpected
		}
		return s.emitRole(txCtx, events.ProductRoleDeleted, input.ProductID, input.ID)
	})
	if err != nil {
		return err
	}

	logger.Info().
		Str("product_role_id", input.ID).
		Str("product_id", input.ProductID).
		Msg("product role deleted")

	return nil
}

// releaseInvitations refuses while a pending invitation names the role. It
// then deletes the accepted and expired invitations that name the role, each
// with its deleted event, so the foreign key cascade never removes one silently.
func (s *productRoleService) releaseInvitations(
	ctx context.Context, productID, roleID string, logger zerolog.Logger,
) error {
	return releaseRoleInvitations(ctx, s.invitationRepo, s.events, productID, roleID, logger)
}

// releaseRoleInvitations refuses while a pending invitation names the role,
// then deletes the accepted and expired invitations that name it, each with
// its deleted event.
func releaseRoleInvitations(
	ctx context.Context,
	invitations invitationrepository.Repository,
	emitter events.Emitter,
	productID, roleID string,
	logger zerolog.Logger,
) error {
	pending, err := invitations.ExistsPendingForRole(ctx, productID, roleID)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Str("role_id", roleID).
			Err(err).
			Msg("failed to check pending invitations of the role")
		return fault.ErrUnexpected
	}
	if pending {
		return NewRoleInUseError(roleID)
	}

	deleted, err := invitations.DeleteNonPendingForRole(ctx, productID, roleID)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Str("role_id", roleID).
			Err(err).
			Msg("failed to delete the settled invitations of the role")
		return fault.ErrUnexpected
	}
	for _, invitation := range deleted {
		if emitErr := emitter.Emit(ctx, events.Event{
			Type:      events.OrganizationInvitationDeleted,
			ProductID: productID,
			Data:      events.InvitationData(invitation.OrganizationID, invitation.ID),
		}); emitErr != nil {
			return emitErr
		}
	}
	return nil
}

func (s *productRoleService) AssignPermissionToProductRole(
	ctx context.Context, input role.AssignPermissionToProductRoleInput,
) error {
	logger := s.logger.With().Str("operation", "AssignPermissionToProductRole").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}
	logger.Info().
		Str("product_role_id", input.ProductRoleID).
		Str("permission_name", input.PermissionName).
		Str("product_id", input.ProductID).
		Msg("assigning permission to product role")

	if err := s.requireRole(ctx, input.ProductID, input.ProductRoleID, logger); err != nil {
		return err
	}

	newPermission := role.ProductRolePermission{
		ProductRoleID:  input.ProductRoleID,
		ProductID:      input.ProductID,
		PermissionName: input.PermissionName,
	}
	newPermission.GenerateID()

	permissions := []role.ProductRolePermission{newPermission}
	resolvedPermissions, err := s.resolveRolePermissions(ctx, input.ProductID, permissions, logger)
	if err != nil {
		return err
	}
	newPermission = resolvedPermissions[0]

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		tree, treeErr := s.resourceTypes.LockedTree(txCtx, input.ProductID)
		if treeErr != nil {
			return treeErr
		}
		if lockErr := s.roleRepo.LockTemplate(txCtx, input.ProductID, input.ProductRoleID); lockErr != nil {
			return lockErr
		}
		target, findErr := s.roleRepo.FindByProductIDAndRoleID(txCtx, input.ProductID, input.ProductRoleID)
		if findErr != nil {
			return findErr
		}
		if target.IsAbsent() {
			return role.ErrProductRoleNotFound
		}
		roleScope := scopeSlug(tree, target.Value().ResourceTypeID)
		if scopeErr := s.checkBundle(txCtx, tree, input.ProductID, roleScope, resolvedPermissions); scopeErr != nil {
			return scopeErr
		}
		added, addErr := s.roleRepo.AddPermission(txCtx, newPermission)
		if addErr != nil {
			logger.Error().
				Str("product_role_id", input.ProductRoleID).
				Str("permission_name", newPermission.PermissionName).
				Err(addErr).
				Msg("failed to add permission to role")
			return fault.ErrUnexpected
		}
		if !added {
			logger.Debug().
				Str("permission_name", newPermission.PermissionName).
				Str("product_role_id", input.ProductRoleID).
				Msg("permission already assigned to role")
			return nil
		}
		logger.Info().
			Str("product_role_id", input.ProductRoleID).
			Str("permission_name", newPermission.PermissionName).
			Msg("permission assigned to product role")
		return s.markRoleUpdated(txCtx, input.ProductID, input.ProductRoleID, logger)
	})
}

func (s *productRoleService) UnassignPermissionFromProductRole(
	ctx context.Context, input role.UnassignPermissionFromProductRoleInput,
) error {
	logger := s.logger.With().Str("operation", "UnassignPermissionFromProductRole").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}
	logger.Info().
		Str("product_role_id", input.ProductRoleID).
		Str("permission_name", input.PermissionName).
		Str("product_id", input.ProductID).
		Msg("unassigning permission from product role")

	if err := s.requireRole(ctx, input.ProductID, input.ProductRoleID, logger); err != nil {
		return err
	}

	foundPermission, err := s.productResourcePermissionRepo.FindByName(
		ctx, input.ProductID, input.PermissionName,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("permission_name", input.PermissionName).
			Err(err).
			Msg("failed to find permission")
		return fault.ErrUnexpected
	}
	if foundPermission.IsAbsent() {
		return NewProductRoleResourcePermissionNotFoundError(
			input.ProductID, input.PermissionName,
		)
	}
	permissionName := foundPermission.Value().Name

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if lockErr := s.roleRepo.LockTemplate(txCtx, input.ProductID, input.ProductRoleID); lockErr != nil {
			return lockErr
		}
		removed, removeErr := s.roleRepo.RemovePermission(
			txCtx, input.ProductID, input.ProductRoleID, permissionName,
		)
		if removeErr != nil {
			logger.Error().
				Str("product_role_id", input.ProductRoleID).
				Str("permission_name", permissionName).
				Err(removeErr).
				Msg("failed to remove permission from role")
			return fault.ErrUnexpected
		}
		if !removed {
			logger.Debug().
				Str("permission_name", permissionName).
				Str("product_role_id", input.ProductRoleID).
				Msg("permission not found in role")
			return nil
		}
		logger.Info().
			Str("product_role_id", input.ProductRoleID).
			Str("permission_name", permissionName).
			Msg("permission unassigned from product role")
		return s.markRoleUpdated(txCtx, input.ProductID, input.ProductRoleID, logger)
	})
}

func (s *productRoleService) requireRole(
	ctx context.Context, productID, roleID string, logger zerolog.Logger,
) error {
	foundRole, err := s.roleRepo.FindByProductIDAndRoleID(ctx, productID, roleID)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Str("role_id", roleID).
			Err(err).
			Msg("failed to find role")
		return fault.ErrUnexpected
	}
	if foundRole.IsAbsent() {
		return role.ErrProductRoleNotFound
	}
	return nil
}

func (s *productRoleService) markRoleUpdated(
	ctx context.Context, productID, roleID string, logger zerolog.Logger,
) error {
	if err := s.organizationRoles.SyncTemplate(ctx, productID, roleID); err != nil {
		return err
	}
	if err := s.roleRepo.TouchUpdatedAt(ctx, productID, roleID); err != nil {
		logger.Error().
			Str("product_id", productID).
			Str("role_id", roleID).
			Err(err).
			Msg("failed to touch role updated_at")
		return fault.ErrUnexpected
	}
	return s.emitRole(ctx, events.ProductRoleUpdated, productID, roleID)
}

func (s *productRoleService) nameDuplicationValidation(
	ctx context.Context, productID, roleName, currentRoleID string, logger zerolog.Logger,
) error {
	// Exact-name lookup. The search filter matches names as substrings, which
	// would wrongly flag e.g. "role" as a duplicate of "role-admin".
	found, err := s.roleRepo.GetByProductIDAndName(ctx, productID, roleName)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Str("role_name", roleName).
			Err(err).
			Msg("failed to look up product role by name")
		return fault.ErrUnexpected
	}
	if found.IsPresent() && found.Value().ID != currentRoleID {
		return NewRoleWithAlreadyExistingNameError(
			roleName, productID,
		)
	}
	return nil
}

func (s *productRoleService) resolveRolePermissions(
	ctx context.Context, productID string, input []role.ProductRolePermission, logger zerolog.Logger,
) ([]role.ProductRolePermission, error) {
	if len(input) == 0 {
		return input, nil
	}

	permissionNames := functional.Slice(input).Map(func(p role.ProductRolePermission) string {
		return p.PermissionName
	})
	canonicalNames, err := resolvePermissionNames(
		ctx,
		s.productResourcePermissionRepo,
		productID,
		permissionNames,
		logger,
	)
	if err != nil {
		return nil, err
	}

	resolved := slices.Clone(input)
	for i, name := range canonicalNames {
		resolved[i].PermissionName = name
	}
	return resolved, nil
}

// resolvePermissionNames checks that every named permission exists in the
// product and returns the names as the product spells them.
func resolvePermissionNames(
	ctx context.Context,
	permissions repository.ProductResourcePermissionRepository,
	productID string,
	permissionNames []string,
	logger zerolog.Logger,
) ([]string, error) {
	if len(permissionNames) == 0 {
		return permissionNames, nil
	}
	inputValidator := struct {
		Permissions []string `validate:"max=100"`
	}{
		Permissions: permissionNames,
	}
	if err := validate.ValidateStruct(inputValidator); err != nil {
		logger.Debug().Err(err).Msg("too many permissions requested")
		return nil, err
	}

	searchReq := search.NewRequest[resourcepermission.SearchProductResourcePermissionFilter, resourcepermission.SortFieldProductResourcePermission]().
		WithFilter(&resourcepermission.SearchProductResourcePermissionFilter{
			Names: permissionNames,
		}).
		WithPagination(&search.Pagination{
			Limit: int32(len(permissionNames)), //nolint:gosec // Checked by validation
		})

	result, err := permissions.SearchByProduct(
		ctx, productID, searchReq,
	)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Err(err).
			Msg("failed to search resource permissions")
		return nil, fault.ErrUnexpected
	}

	foundNames := functional.Slice(result.Items).Map(func(p resourcepermission.ProductResourcePermission) string {
		return p.Name
	})
	canonicalNames, missing := canonicalizePermissionNames(foundNames, permissionNames)
	if len(missing) > 0 {
		return nil, NewPermissionsNotFoundError(productID, missing)
	}
	return canonicalNames, nil
}

func (s *productRoleService) withScope(
	ctx context.Context, productID string, found role.ProductRole, logger zerolog.Logger,
) (role.ProductRole, error) {
	tree, err := s.resourceTypes.Tree(ctx, productID)
	if err != nil {
		logger.Error().Str("product_id", productID).Err(err).Msg("failed to read resource types")
		return role.ProductRole{}, fault.ErrUnexpected
	}
	return withRoleScope(tree)(found), nil
}

// checkBundle reads the scopes of the role's permissions and refuses one
// outside the role's scope.
func (s *productRoleService) checkBundle(
	ctx context.Context, tree resourcetype.Tree, productID, roleScope string,
	permissions []role.ProductRolePermission,
) error {
	if len(permissions) == 0 {
		return nil
	}
	names := functional.Slice(permissions).Map(func(p role.ProductRolePermission) string { return p.PermissionName })
	return checkNamedBundle(ctx, s.productResourcePermissionRepo, tree, productID, roleScope, names)
}

// checkNamedBundle reads the scopes of the named permissions and refuses one
// outside the role's scope.
func checkNamedBundle(
	ctx context.Context,
	permissions repository.ProductResourcePermissionRepository,
	tree resourcetype.Tree,
	productID, roleScope string,
	names []string,
) error {
	if len(names) == 0 {
		return nil
	}
	found, err := permissions.FindByProductIDAndPermissionNames(ctx, productID, names)
	if err != nil {
		return err
	}
	return checkBundleScope(tree, roleScope, found)
}

// takeDefault locks the Product's current default template with the template
// being written, in ID order, and clears it so that the template can take
// the default. Only an organization-scoped template can. It returns the
// templates it cleared, other than the one being written.
func (s *productRoleService) takeDefault(ctx context.Context, productID, id, scope string) ([]string, error) {
	if scope != resourcetype.OrganizationSlug {
		return nil, role.ErrDefaultNotOrganizationScoped
	}
	current, err := s.roleRepo.DefaultTemplateID(ctx, productID)
	if err != nil {
		return nil, err
	}
	locked := current.Map(func(d string) []string { return []string{d} }).OrElse(nil)
	if id != "" {
		locked = append(locked, id)
	}
	if err = s.roleRepo.LockTemplates(ctx, productID, locked); err != nil {
		return nil, err
	}
	cleared, err := s.roleRepo.ClearDefault(ctx, productID)
	return functional.Slice(cleared).Filter(func(c string) bool { return c != id }), err
}

// applyTemplateDefault locks the template, with the current default when the
// update takes the default, and writes is_default only when the update names
// it. It returns the templates whose default it cleared.
func (s *productRoleService) applyTemplateDefault(
	ctx context.Context, input role.UpdateProductRoleInput, scope string,
) ([]string, error) {
	if input.IsDefault == nil || !*input.IsDefault {
		if err := s.roleRepo.LockTemplate(ctx, input.ProductID, input.ID); err != nil {
			return nil, err
		}
		if input.IsDefault == nil {
			return nil, nil
		}
		return nil, s.roleRepo.SetDefault(ctx, input.ProductID, input.ID, false)
	}
	cleared, err := s.takeDefault(ctx, input.ProductID, input.ID, scope)
	if err != nil {
		return nil, err
	}
	err = s.roleRepo.SetDefault(ctx, input.ProductID, input.ID, true)
	if errors.Is(err, repository.ErrProductRoleDefaultExists) {
		return nil, organizationrole.ErrDefaultConflict
	}
	return cleared, err
}

func (s *productRoleService) emitRoles(ctx context.Context, productID string, roleIDs []string) error {
	for _, roleID := range roleIDs {
		if err := s.emitRole(ctx, events.ProductRoleUpdated, productID, roleID); err != nil {
			return err
		}
	}
	return nil
}

func (s *productRoleService) keyDuplicationValidation(
	ctx context.Context, productID, name, key string, logger zerolog.Logger,
) error {
	found, err := s.roleRepo.GetByProductIDAndKey(ctx, productID, key)
	if err != nil {
		logger.Error().Str("product_id", productID).Err(err).Msg("failed to look up product role by key")
		return fault.ErrUnexpected
	}
	if found.IsPresent() {
		return s.keyConflict(ctx, productID, name, key, logger)
	}
	return nil
}

// claimTemplateKey holds the key for the transaction and refuses one that a
// custom role of any Organization holds: every Organization's copy of the
// template would take it.
func (s *productRoleService) claimTemplateKey(ctx context.Context, productID, key string) error {
	if err := s.organizationRoles.LockKey(ctx, productID, key); err != nil {
		return err
	}
	taken, err := s.organizationRoles.CustomKeyTaken(ctx, productID, key)
	if err != nil {
		return err
	}
	if taken {
		return role.NewKeyExistsError(key)
	}
	return nil
}

// detachWhenCustomRolesOn keeps every member's access when the Product lets
// Organizations write their own roles: the copies of the template that are
// held, named by a pending invitation or customized become custom roles of
// their Organization (ADR-0022 D10.2). With the switch off, a held template
// still answers 409 ROLE_IN_USE.
func (s *productRoleService) detachWhenCustomRolesOn(ctx context.Context, productID, templateID string) error {
	tree, err := s.resourceTypes.Tree(ctx, productID)
	if err != nil {
		return err
	}
	if !customRolesOn(tree, resourcetype.OrganizationSlug) {
		return nil
	}
	return s.organizationRoles.DetachCopies(ctx, productID, templateID)
}

// keyConflict names the conflict of a create whose key is taken. A key
// defaults to the name in lower case, so a racing create of the same name can
// meet the key first; it still answers the name conflict.
func (s *productRoleService) keyConflict(
	ctx context.Context, productID, name, key string, logger zerolog.Logger,
) error {
	if err := s.nameDuplicationValidation(ctx, productID, name, "", logger); err != nil {
		return err
	}
	return role.NewKeyExistsError(key)
}

// mapTemplateWriteError turns the unique guards a template write can trip
// into the conflicts their pre-checks answer; any other error passes through.
func mapTemplateWriteError(err error, name, key, productID string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrProductRoleNameExists):
		return NewRoleWithAlreadyExistingNameError(name, productID)
	case errors.Is(err, repository.ErrProductRoleKeyExists):
		return role.NewKeyExistsError(key)
	case errors.Is(err, repository.ErrProductRoleDefaultExists):
		return organizationrole.ErrDefaultConflict
	default:
		return nil
	}
}

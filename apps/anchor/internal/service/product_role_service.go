package service

import (
	"context"
	"errors"
	"slices"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	resourcepermission "anchor/internal/domain/product/resource_permission"
	role "anchor/internal/domain/product/role"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/events"
	invitationrepository "anchor/internal/invitation/repository"
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
	transactor                    transactor.Transactor
	events                        events.Emitter
	logger                        zerolog.Logger
}

func NewProductRoleService(
	roleRepo repository.ProductRoleRepository,
	productResourcePermissionRepo repository.ProductResourcePermissionRepository,
	invitationRepo invitationrepository.Repository,
	resourceTypes rtservice.Service,
	tx transactor.Transactor,
	eventEmitter events.Emitter,
	logger zerolog.Logger,
) ProductRoleService {
	return &productRoleService{
		roleRepo:                      roleRepo,
		productResourcePermissionRepo: productResourcePermissionRepo,
		invitationRepo:                invitationRepo,
		resourceTypes:                 resourceTypes,
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
	productRole := role.NewManaged(input.ProductID, input.Name, input.Description)
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
		if scopeErr := s.checkPermissionScopes(
			txCtx,
			tree,
			input.ProductID,
			scope.Slug,
			productRole.Permissions,
		); scopeErr != nil {
			return scopeErr
		}
		productRole.ResourceTypeID = scope.ID

		var createErr error
		created, createErr = s.roleRepo.Create(txCtx, productRole)
		if errors.Is(createErr, repository.ErrProductRoleNameExists) {
			return NewRoleWithAlreadyExistingNameError(input.Name, input.ProductID)
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
		return s.emitRole(txCtx, events.ProductRoleCreated, input.ProductID, created.ID)
	})
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
		var updateErr error
		updated, updateErr = s.roleRepo.Update(txCtx, updatedRole)
		if errors.Is(updateErr, repository.ErrProductRoleNameExists) {
			return NewRoleWithAlreadyExistingNameError(updatedRole.Name, input.ProductID)
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
			tree, treeErr := s.resourceTypes.LockedTree(txCtx, input.ProductID)
			if treeErr != nil {
				return treeErr
			}
			if scopeErr := s.checkPermissionScopes(
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
		// FOR UPDATE, not FOR NO KEY UPDATE: a membership written on the role
		// takes FOR KEY SHARE through its foreign key, so it waits for this
		// delete instead of being removed by its cascade.
		exists, lockErr := s.roleRepo.ExistsManaged(transactor.ForUpdate(txCtx), input.ProductID, input.ID)
		if lockErr != nil {
			return lockErr
		}
		if !exists {
			return role.ErrProductRoleNotFound
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
	pending, err := s.invitationRepo.ExistsPendingForRole(ctx, productID, roleID)
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

	deleted, err := s.invitationRepo.DeleteNonPendingForRole(ctx, productID, roleID)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Str("role_id", roleID).
			Err(err).
			Msg("failed to delete the settled invitations of the role")
		return fault.ErrUnexpected
	}
	for _, invitation := range deleted {
		if emitErr := s.events.Emit(ctx, events.Event{
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
		target, findErr := s.roleRepo.FindByProductIDAndRoleID(txCtx, input.ProductID, input.ProductRoleID)
		if findErr != nil {
			return findErr
		}
		if target.IsAbsent() {
			return role.ErrProductRoleNotFound
		}
		roleScope := scopeSlug(tree, target.Value().ResourceTypeID)
		if scopeErr := s.checkPermissionScopes(
			txCtx,
			tree,
			input.ProductID,
			roleScope,
			resolvedPermissions,
		); scopeErr != nil {
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

	result, err := s.productResourcePermissionRepo.SearchByProduct(
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

	resolved := slices.Clone(input[:len(canonicalNames)]) // repeats dropped; entries differ only by name and ID
	for i, name := range canonicalNames {
		resolved[i].PermissionName = name
	}
	return resolved, nil
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

// checkPermissionScopes reads the scopes of the role's permissions and refuses one
// outside the role's scope.
func (s *productRoleService) checkPermissionScopes(
	ctx context.Context, tree resourcetype.Tree, productID, roleScope string,
	permissions []role.ProductRolePermission,
) error {
	if len(permissions) == 0 {
		return nil
	}
	names := functional.Slice(permissions).Map(func(p role.ProductRolePermission) string { return p.PermissionName })
	found, err := s.productResourcePermissionRepo.FindByProductIDAndPermissionNames(ctx, productID, names)
	if err != nil {
		return err
	}
	return checkPermissionScopes(tree, roleScope, found)
}

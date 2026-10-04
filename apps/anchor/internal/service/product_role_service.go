package service

import (
	"context"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	resourcepermission "anchor/internal/domain/product/resource_permission"
	role "anchor/internal/domain/product/role"
	"anchor/internal/events"
	invitationrepository "anchor/internal/invitation/repository"
	"anchor/internal/repository"

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
	transactor                    transactor.Transactor
	events                        events.Emitter
	logger                        zerolog.Logger
}

func NewProductRoleService(
	roleRepo repository.ProductRoleRepository,
	productResourcePermissionRepo repository.ProductResourcePermissionRepository,
	invitationRepo invitationrepository.Repository,
	tx transactor.Transactor,
	eventEmitter events.Emitter,
	logger zerolog.Logger,
) ProductRoleService {
	return &productRoleService{
		roleRepo:                      roleRepo,
		productResourcePermissionRepo: productResourcePermissionRepo,
		invitationRepo:                invitationRepo,
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
	if err := s.nameDuplicationValidation(ctx, input.ProductID, input.Name, "", logger); err != nil {
		return role.ProductRole{}, err
	}
	productRole := role.ProductRole{
		ProductID:   input.ProductID,
		Name:        input.Name,
		Description: input.Description,
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

	if err := s.permissionsValidation(ctx, input.ProductID, productRolePermission, logger); err != nil {
		return role.ProductRole{}, err
	}

	productRole.Permissions = productRolePermission

	var created role.ProductRole
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.roleRepo.Create(txCtx, productRole)
		if createErr != nil {
			log.Event(&logger, createErr).
				Str("product_id", input.ProductID).
				Str("name", input.Name).
				Msg("failed to create product role")
			return fault.ErrUnexpected
		}
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

	if err := validate.ValidateStruct(input); err != nil {
		return search.Result[role.ProductRole]{}, err
	}

	result, err := s.roleRepo.SearchByProductID(ctx, input.ProductID, input.Request)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Msg("failed to search product roles")
		return search.Result[role.ProductRole]{}, fault.ErrUnexpected
	}

	return result, nil
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
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Msg("failed to get product role")
		return role.ProductRole{}, fault.ErrUnexpected
	}

	return found.ToResult(role.ErrProductRoleNotFound).Value()
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
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Msg("failed to find product role for update")
		return role.ProductRole{}, fault.ErrUnexpected
	}
	if foundRole.IsAbsent() {
		return role.ProductRole{}, role.ErrProductRoleNotFound
	}

	updatedRole := foundRole.Value()

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
		if err = s.permissionsValidation(ctx, input.ProductID, input.Permissions, logger); err != nil {
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
		if updateErr != nil {
			log.Event(&logger, updateErr).
				Str("product_id", input.ProductID).
				Str("role_id", input.ID).
				Msg("failed to update product role")
			return fault.ErrUnexpected
		}
		if input.Permissions != nil {
			if updated, updateErr = s.replacePermissions(txCtx, input, logger); updateErr != nil {
				return updateErr
			}
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
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Msg("failed to replace product role permissions")
		return role.ProductRole{}, fault.ErrUnexpected
	}
	found, err := s.roleRepo.FindByProductIDAndRoleID(ctx, input.ProductID, input.ID)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
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
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Msg("failed to find product role for deletion")
		return fault.ErrUnexpected
	}
	if foundRole.IsAbsent() {
		return role.ErrProductRoleNotFound
	}

	assignmentCount, err := s.roleRepo.CountMembershipAssignments(ctx, input.ID)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Msg("failed to count role membership assignments")
		return fault.ErrUnexpected
	}
	if assignmentCount > 0 {
		return NewRoleInUseError(input.ID)
	}

	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if invitationErr := s.releaseInvitations(txCtx, input.ProductID, input.ID, logger); invitationErr != nil {
			return invitationErr
		}
		if deleteErr := s.roleRepo.DeleteByProductIDAndRoleID(
			txCtx, input.ProductID, input.ID,
		); deleteErr != nil {
			log.Event(&logger, deleteErr).
				Str("product_id", input.ProductID).
				Str("role_id", input.ID).
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
		log.Event(&logger, err).
			Str("product_id", productID).
			Str("role_id", roleID).
			Msg("failed to check pending invitations of the role")
		return fault.ErrUnexpected
	}
	if pending {
		return NewRoleInUseError(roleID)
	}

	deleted, err := s.invitationRepo.DeleteNonPendingForRole(ctx, productID, roleID)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", productID).
			Str("role_id", roleID).
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
	if err := s.permissionsValidation(ctx, input.ProductID, permissions, logger); err != nil {
		return err
	}
	newPermission = permissions[0]

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		added, addErr := s.roleRepo.AddPermission(txCtx, newPermission)
		if addErr != nil {
			log.Event(&logger, addErr).
				Str("product_role_id", input.ProductRoleID).
				Str("permission_name", newPermission.PermissionName).
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
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("permission_name", input.PermissionName).
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
			log.Event(&logger, removeErr).
				Str("product_role_id", input.ProductRoleID).
				Str("permission_name", permissionName).
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
		log.Event(&logger, err).
			Str("product_id", productID).
			Str("role_id", roleID).
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
		log.Event(&logger, err).
			Str("product_id", productID).
			Str("role_id", roleID).
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
		log.Event(&logger, err).
			Str("product_id", productID).
			Str("role_name", roleName).
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

func (s *productRoleService) permissionsValidation(
	ctx context.Context, productID string, input []role.ProductRolePermission, logger zerolog.Logger,
) error {
	if len(input) == 0 {
		return nil
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
		return err
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
		log.Event(&logger, err).
			Str("product_id", productID).
			Msg("failed to search resource permissions")
		return fault.ErrUnexpected
	}

	foundNames := functional.Slice(result.Items).Map(func(p resourcepermission.ProductResourcePermission) string {
		return p.Name
	})
	foundMap := functional.Slice(foundNames).ToMap(strings.ToLower)

	var notFoundPermissions []string
	for i := range input {
		name := input[i].PermissionName
		canonicalName, ok := foundMap[strings.ToLower(name)]
		if !ok {
			notFoundPermissions = append(notFoundPermissions, name)
			continue
		}
		input[i].PermissionName = canonicalName
	}

	if len(notFoundPermissions) > 0 {
		return NewPermissionsNotFoundError(productID, notFoundPermissions)
	}

	return nil
}

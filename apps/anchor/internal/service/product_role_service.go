package service

import (
	"context"

	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"

	resourcepermission "anchor/internal/domain/product/resource_permission"
	role "anchor/internal/domain/product/role"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
)

type ProductRoleService interface {
	CreateProductRole(
		ctx context.Context, input role.CreateProductRoleInput,
	) (role.ProductRole, error)
	SearchProductRoles(
		ctx context.Context, input role.SearchProductRolesInput,
	) (search.Result[role.ProductRole], error)
	GetProductRole(
		ctx context.Context, input role.GetProductRoleInput,
	) (*role.ProductRole, error)
	UpdateProductRole(
		ctx context.Context, input role.UpdateProductRoleInput,
	) (role.ProductRole, error)
	DeleteProductRole(
		ctx context.Context, input role.DeleteProductRoleInput,
	) error
	AssignPermissionToProductRole(
		ctx context.Context, input role.AssignPermissionToProductRoleInput,
	) (role.ProductRole, error)
	UnassignPermissionFromProductRole(
		ctx context.Context, input role.UnassignPermissionFromProductRoleInput,
	) (role.ProductRole, error)
}

type productRoleService struct {
	roleRepo                      repository.ProductRoleRepository
	productResourcePermissionRepo repository.ProductResourcePermissionRepository
	logger                        zerolog.Logger
}

func NewProductRoleService(
	roleRepo repository.ProductRoleRepository,
	productResourcePermissionRepo repository.ProductResourcePermissionRepository,
	logger zerolog.Logger,
) ProductRoleService {
	return &productRoleService{
		roleRepo:                      roleRepo,
		productResourcePermissionRepo: productResourcePermissionRepo,
		logger: logger.With().Str(
			"component", "product_role_permission_service",
		).Logger(),
	}
}

func (s *productRoleService) CreateProductRole(
	ctx context.Context, input role.CreateProductRoleInput,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "CreateProductRole").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return role.ProductRole{}, err
	}
	if err := s.nameDuplicationValidation(ctx, input.ProductID, input.Name, logger); err != nil {
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

	productRolePermission := toolkit.TransformSlice(
		input.Permissions, func(perm string) role.ProductRolePermission {
			p := role.ProductRolePermission{
				ProductRoleID:  productRole.ID,
				ProductID:      input.ProductID,
				PermissionName: perm,
			}
			p.GenerateID()
			return p
		},
	)
	if err := s.permissionsValidation(ctx, input.ProductID, productRolePermission, logger); err != nil {
		return role.ProductRole{}, err
	}

	productRole.Permissions = productRolePermission

	created, err := s.roleRepo.Create(ctx, productRole, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to create product role")
		return role.ProductRole{}, toolkit.ErrUnexpected
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

	if err := toolkit.ValidateStruct(input); err != nil {
		return search.Result[role.ProductRole]{}, err
	}

	result, err := s.roleRepo.SearchByProductID(ctx, input.ProductID, input.Request, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to search product roles")
		return search.Result[role.ProductRole]{}, toolkit.ErrUnexpected
	}

	return result, nil
}

func (s *productRoleService) GetProductRole(
	ctx context.Context, input role.GetProductRoleInput,
) (*role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "GetProductRole").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}

	productRole, err := s.roleRepo.FindByProductIDAndRoleID(ctx, input.ProductID, input.ID, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to get product role")
		return nil, toolkit.ErrUnexpected
	}

	return productRole, nil
}

func (s *productRoleService) UpdateProductRole(
	ctx context.Context, input role.UpdateProductRoleInput,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "UpdateProductRole").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return role.ProductRole{}, err
	}

	logger.Info().
		Str("product_role_id", input.ID).
		Str("product_id", input.ProductID).
		Msg("updating product role")

	existingRole, err := s.roleRepo.FindByProductIDAndRoleID(ctx, input.ProductID, input.ID, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to find product role for update")
		return role.ProductRole{}, toolkit.ErrUnexpected
	}
	if existingRole == nil {
		return role.ProductRole{}, toolkit.ErrNotFound
	}

	updatedRole := *existingRole

	if input.Name != nil && *input.Name != updatedRole.Name {
		err = s.nameDuplicationValidation(ctx, input.ProductID, *input.Name, logger)
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
		updatedRole.Permissions = input.Permissions
	}

	updated, err := s.roleRepo.Update(ctx, updatedRole, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to update product role")
		return role.ProductRole{}, toolkit.ErrUnexpected
	}

	logger.Info().
		Str("product_role_id", input.ID).
		Str("product_id", input.ProductID).
		Msg("product role updated")

	return updated, nil
}

func (s *productRoleService) DeleteProductRole(
	ctx context.Context, input role.DeleteProductRoleInput,
) error {
	logger := s.logger.With().Str("operation", "DeleteProductRole").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return err
	}

	existingRole, err := s.roleRepo.FindByProductIDAndRoleID(ctx, input.ProductID, input.ID, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to find product role for deletion")
		return toolkit.ErrUnexpected
	}
	if existingRole == nil {
		return toolkit.ErrNotFound
	}

	assignmentCount, err := s.roleRepo.CountMembershipAssignments(ctx, input.ID, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to count role membership assignments")
		return toolkit.ErrUnexpected
	}
	if assignmentCount > 0 {
		return NewRoleInUseError(input.ID)
	}

	err = s.roleRepo.DeleteByProductIDAndRoleID(ctx, input.ProductID, input.ID, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to delete product role")
		return toolkit.ErrUnexpected
	}

	logger.Info().
		Str("product_role_id", input.ID).
		Str("product_id", input.ProductID).
		Msg("product role deleted")

	return nil
}

func (s *productRoleService) AssignPermissionToProductRole(
	ctx context.Context, input role.AssignPermissionToProductRoleInput,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "AssignPermissionToProductRole").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return role.ProductRole{}, err
	}
	logger.Info().
		Str("product_role_id", input.ProductRoleID).
		Str("permission_name", input.PermissionName).
		Str("product_id", input.ProductID).
		Msg("assigning permission to product role")

	productRole, err := s.roleRepo.FindByProductIDAndRoleID(
		ctx, input.ProductID, input.ProductRoleID, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ProductRoleID).
			Err(err).
			Msg("failed to find role")
		return role.ProductRole{}, toolkit.ErrUnexpected
	}
	if productRole == nil {
		return role.ProductRole{}, NewRoleNotFoundError(input.ProductRoleID)
	}

	newPermission := role.ProductRolePermission{
		ProductRoleID:  input.ProductRoleID,
		ProductID:      input.ProductID,
		PermissionName: input.PermissionName,
	}
	newPermission.GenerateID()

	if err = s.permissionsValidation(
		ctx, input.ProductID,
		[]role.ProductRolePermission{
			newPermission,
		},
		logger,
	); err != nil {
		return role.ProductRole{}, err
	}

	for _, perm := range productRole.Permissions {
		if perm.PermissionName == input.PermissionName {
			logger.Debug().
				Str("permission_name", input.PermissionName).
				Str("product_role_id", input.ProductRoleID).
				Msg("permission already assigned to role")
			return *productRole, nil
		}
	}

	productRole.Permissions = append(productRole.Permissions, newPermission)

	updated, err := s.roleRepo.Update(ctx, *productRole, nil)
	if err != nil {
		logger.Error().
			Str("product_role_id", input.ProductRoleID).
			Str("permission_name", input.PermissionName).
			Err(err).
			Msg("failed to update role with new permission")
		return role.ProductRole{}, toolkit.ErrUnexpected
	}

	logger.Info().
		Str("product_role_id", input.ProductRoleID).
		Str("permission_name", input.PermissionName).
		Msg("permission assigned to product role")

	return updated, nil
}

func (s *productRoleService) UnassignPermissionFromProductRole(
	ctx context.Context, input role.UnassignPermissionFromProductRoleInput,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "UnassignPermissionFromProductRole").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return role.ProductRole{}, err
	}
	logger.Info().
		Str("product_role_id", input.ProductRoleID).
		Str("permission_name", input.PermissionName).
		Str("product_id", input.ProductID).
		Msg("unassigning permission from product role")

	productRole, err := s.roleRepo.FindByProductIDAndRoleID(
		ctx, input.ProductID, input.ProductRoleID, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ProductRoleID).
			Err(err).
			Msg("failed to find role")
		return role.ProductRole{}, toolkit.ErrUnexpected
	}
	if productRole == nil {
		return role.ProductRole{}, NewRoleNotFoundError(input.ProductRoleID)
	}

	permissionFound, err := s.productResourcePermissionRepo.FindByName(
		ctx, input.ProductID, input.PermissionName, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("permission_name", input.PermissionName).
			Err(err).
			Msg("failed to find permission")
		return role.ProductRole{}, toolkit.ErrUnexpected
	}
	if permissionFound == nil {
		return role.ProductRole{}, NewPermissionsNotFoundError(
			input.ProductID, []string{input.PermissionName},
		)
	}

	found := false
	var updatedPermissions []role.ProductRolePermission
	for _, perm := range productRole.Permissions {
		if perm.PermissionName != input.PermissionName {
			updatedPermissions = append(updatedPermissions, perm)
		} else {
			found = true
		}
	}

	if !found {
		logger.Debug().
			Str("permission_name", input.PermissionName).
			Str("product_role_id", input.ProductRoleID).
			Msg("permission not found in role")
		return *productRole, nil
	}

	productRole.Permissions = updatedPermissions

	updated, err := s.roleRepo.Update(ctx, *productRole, nil)
	if err != nil {
		logger.Error().
			Str("product_role_id", input.ProductRoleID).
			Str("permission_name", input.PermissionName).
			Err(err).
			Msg("failed to update role after removing permission")
		return role.ProductRole{}, toolkit.ErrUnexpected
	}

	logger.Info().
		Str("product_role_id", input.ProductRoleID).
		Str("permission_name", input.PermissionName).
		Msg("permission unassigned from product role")

	return updated, nil
}

func (s *productRoleService) nameDuplicationValidation(
	ctx context.Context, productID, roleName string, logger zerolog.Logger,
) error {
	productRoles, err := s.roleRepo.SearchByProductID(
		ctx, productID, search.NewRequest[role.SearchProductRoleFilter,
			role.SortFieldProductRole]().WithFilter(
			&role.SearchProductRoleFilter{
				Names: []string{roleName},
			},
		).One(),
		nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Str("role_name", roleName).
			Err(err).
			Msg("failed to search for product roles")
		return toolkit.ErrUnexpected
	}
	if productRoles.Count > 0 {
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

	permissionNames := toolkit.TransformSlice(input, func(p role.ProductRolePermission) string {
		return p.PermissionName
	})

	inputValidator := struct {
		Permissions []string `validate:"max=100"`
	}{
		Permissions: permissionNames,
	}
	if err := toolkit.ValidateStruct(inputValidator); err != nil {
		logger.Error().Err(err).Msg("too many permissions requested")
		return toolkit.ErrUnexpected
	}

	searchReq := search.NewRequest[resourcepermission.SearchProductResourcePermissionFilter, resourcepermission.SortFieldProductResourcePermission]().
		WithFilter(&resourcepermission.SearchProductResourcePermissionFilter{
			Names: permissionNames,
		}).
		WithPagination(&search.Pagination{
			Limit: int32(len(permissionNames)), //nolint:gosec // Checked by validation
		})

	result, err := s.productResourcePermissionRepo.SearchByProduct(
		ctx, productID, searchReq, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Err(err).
			Msg("failed to search resource permissions")
		return toolkit.ErrUnexpected
	}

	foundMap := make(map[string]bool)
	for _, p := range result.Items {
		foundMap[p.Name] = true
	}

	var notFoundPermissions []string
	for _, name := range permissionNames {
		if !foundMap[name] {
			notFoundPermissions = append(notFoundPermissions, name)
		}
	}

	if len(notFoundPermissions) > 0 {
		return NewPermissionsNotFoundError(productID, notFoundPermissions)
	}

	return nil
}

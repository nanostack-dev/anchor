package service

import (
	"context"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	resourcepermission "anchor/internal/domain/product/resource_permission"
	role "anchor/internal/domain/product/role"
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
	return fault.NotFound("RESOURCE_PERMISSION_NOT_FOUND", "Resource permission does not exist").
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

	if err := validateStruct(input); err != nil {
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

	productRolePermission := slicex.Map(
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

	created, err := s.roleRepo.Create(ctx, productRole)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to create product role")
		return role.ProductRole{}, fault.ErrUnexpected
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

	if err := validateStruct(input); err != nil {
		return search.Result[role.ProductRole]{}, err
	}

	result, err := s.roleRepo.SearchByProductID(ctx, input.ProductID, input.Request)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to search product roles")
		return search.Result[role.ProductRole]{}, fault.ErrUnexpected
	}

	return result, nil
}

func (s *productRoleService) GetProductRole(
	ctx context.Context, input role.GetProductRoleInput,
) (*role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "GetProductRole").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	found, err := s.roleRepo.FindByProductIDAndRoleID(ctx, input.ProductID, input.ID)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to get product role")
		return nil, fault.ErrUnexpected
	}

	return found.ToPtr(), nil
}

func (s *productRoleService) UpdateProductRole(
	ctx context.Context, input role.UpdateProductRoleInput,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "UpdateProductRole").Logger()

	if err := validateStruct(input); err != nil {
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
		return role.ProductRole{}, fault.ErrNotFound
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
		updatedRole.Permissions = input.Permissions
	}

	updated, err := s.roleRepo.Update(ctx, updatedRole)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to update product role")
		return role.ProductRole{}, fault.ErrUnexpected
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

	if err := validateStruct(input); err != nil {
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
		return fault.ErrNotFound
	}

	assignmentCount, err := s.roleRepo.CountMembershipAssignments(ctx, input.ID)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to count role membership assignments")
		return fault.ErrUnexpected
	}
	if assignmentCount > 0 {
		return NewRoleInUseError(input.ID)
	}

	err = s.roleRepo.DeleteByProductIDAndRoleID(ctx, input.ProductID, input.ID)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ID).
			Err(err).
			Msg("failed to delete product role")
		return fault.ErrUnexpected
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

	if err := validateStruct(input); err != nil {
		return role.ProductRole{}, err
	}
	logger.Info().
		Str("product_role_id", input.ProductRoleID).
		Str("permission_name", input.PermissionName).
		Str("product_id", input.ProductID).
		Msg("assigning permission to product role")

	foundRole, err := s.roleRepo.FindByProductIDAndRoleID(
		ctx, input.ProductID, input.ProductRoleID,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ProductRoleID).
			Err(err).
			Msg("failed to find role")
		return role.ProductRole{}, fault.ErrUnexpected
	}
	if foundRole.IsAbsent() {
		return role.ProductRole{}, NewRoleNotFoundError(input.ProductRoleID)
	}
	productRole := foundRole.ToPtr()

	newPermission := role.ProductRolePermission{
		ProductRoleID:  input.ProductRoleID,
		ProductID:      input.ProductID,
		PermissionName: input.PermissionName,
	}
	newPermission.GenerateID()

	permissions := []role.ProductRolePermission{newPermission}
	if err = s.permissionsValidation(ctx, input.ProductID, permissions, logger); err != nil {
		return role.ProductRole{}, err
	}
	newPermission = permissions[0]

	for _, perm := range productRole.Permissions {
		if strings.EqualFold(perm.PermissionName, newPermission.PermissionName) {
			logger.Debug().
				Str("permission_name", newPermission.PermissionName).
				Str("product_role_id", input.ProductRoleID).
				Msg("permission already assigned to role")
			return *productRole, nil
		}
	}

	productRole.Permissions = append(productRole.Permissions, newPermission)

	updated, err := s.roleRepo.Update(ctx, *productRole)
	if err != nil {
		logger.Error().
			Str("product_role_id", input.ProductRoleID).
			Str("permission_name", input.PermissionName).
			Err(err).
			Msg("failed to update role with new permission")
		return role.ProductRole{}, fault.ErrUnexpected
	}

	logger.Info().
		Str("product_role_id", input.ProductRoleID).
		Str("permission_name", newPermission.PermissionName).
		Msg("permission assigned to product role")

	return updated, nil
}

func (s *productRoleService) UnassignPermissionFromProductRole(
	ctx context.Context, input role.UnassignPermissionFromProductRoleInput,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "UnassignPermissionFromProductRole").Logger()

	if err := validateStruct(input); err != nil {
		return role.ProductRole{}, err
	}
	logger.Info().
		Str("product_role_id", input.ProductRoleID).
		Str("permission_name", input.PermissionName).
		Str("product_id", input.ProductID).
		Msg("unassigning permission from product role")

	foundRole, err := s.roleRepo.FindByProductIDAndRoleID(
		ctx, input.ProductID, input.ProductRoleID,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("role_id", input.ProductRoleID).
			Err(err).
			Msg("failed to find role")
		return role.ProductRole{}, fault.ErrUnexpected
	}
	if foundRole.IsAbsent() {
		return role.ProductRole{}, NewRoleNotFoundError(input.ProductRoleID)
	}
	productRole := foundRole.ToPtr()

	foundPermission, err := s.productResourcePermissionRepo.FindByName(
		ctx, input.ProductID, input.PermissionName,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("permission_name", input.PermissionName).
			Err(err).
			Msg("failed to find permission")
		return role.ProductRole{}, fault.ErrUnexpected
	}
	if foundPermission.IsAbsent() {
		// input.PermissionName is unassignPermissionFromProductRole's
		// permission_id path segment, not a body-supplied list, so this
		// answers 404, unlike the permissionsValidation bulk check below.
		return role.ProductRole{}, NewProductRoleResourcePermissionNotFoundError(
			input.ProductID, input.PermissionName,
		)
	}
	permissionFound := foundPermission.Value()

	found := false
	var updatedPermissions []role.ProductRolePermission
	for _, perm := range productRole.Permissions {
		if !strings.EqualFold(perm.PermissionName, permissionFound.Name) {
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

	updated, err := s.roleRepo.Update(ctx, *productRole)
	if err != nil {
		logger.Error().
			Str("product_role_id", input.ProductRoleID).
			Str("permission_name", input.PermissionName).
			Err(err).
			Msg("failed to update role after removing permission")
		return role.ProductRole{}, fault.ErrUnexpected
	}

	logger.Info().
		Str("product_role_id", input.ProductRoleID).
		Str("permission_name", input.PermissionName).
		Msg("permission unassigned from product role")

	return updated, nil
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

func (s *productRoleService) permissionsValidation(
	ctx context.Context, productID string, input []role.ProductRolePermission, logger zerolog.Logger,
) error {
	if len(input) == 0 {
		return nil
	}

	permissionNames := slicex.Map(input, func(p role.ProductRolePermission) string {
		return p.PermissionName
	})

	inputValidator := struct {
		Permissions []string `validate:"max=100"`
	}{
		Permissions: permissionNames,
	}
	if err := validateStruct(inputValidator); err != nil {
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
		logger.Error().
			Str("product_id", productID).
			Err(err).
			Msg("failed to search resource permissions")
		return fault.ErrUnexpected
	}

	foundMap := make(map[string]string)
	for _, p := range result.Items {
		foundMap[strings.ToLower(p.Name)] = p.Name
	}

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

package service

import (
	"context"

	"github.com/rs/zerolog"

	role "anchor/internal/domain/product/role"
	"anchor/internal/repository"
)

// Test-only re-exports so the external service_test package can exercise
// unexported helpers without widening the production API. The testpackage
// linter skips export_test.go by design.
var (
	ReapLogLevel              = reapLogLevel
	BuildOrganizationMetadata = buildOrganizationMetadata
)

// ResolveRolePermissions exercises the private resolver with its catalog boundary.
func ResolveRolePermissions(
	ctx context.Context,
	productID string,
	input []role.ProductRolePermission,
	catalog repository.ProductResourcePermissionRepository,
) ([]role.ProductRolePermission, error) {
	s := productRoleService{productResourcePermissionRepo: catalog}
	return s.resolveRolePermissions(ctx, productID, input, zerolog.Nop())
}

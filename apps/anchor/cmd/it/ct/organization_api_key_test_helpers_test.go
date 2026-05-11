package ct_test

import (
	"testing"

	itdsl "anchor/cmd/it/shared/dsl"
)

const (
	organizationAPIKeyPermissionFileRead   = "file:read"
	organizationAPIKeyPermissionFileCreate = "file:create"
	organizationAPIKeyPermissionFileUpdate = "file:update"
	organizationAPIKeyPermissionFileDelete = "file:delete"
)

type organizationAPIKeyResourcePermissions struct {
	FileRead   string
	FileCreate string
	FileUpdate string
	FileDelete string
}

func givenOrganizationAPIKeyResourcePermissions(
	t *testing.T,
	product *itdsl.ProductContext,
) organizationAPIKeyResourcePermissions {
	t.Helper()

	createdPermissions := product.CreateDefaultProductResourcePermissions(t)

	permissionNames := make(map[string]bool, len(createdPermissions))
	for _, permission := range createdPermissions {
		permissionNames[permission.Name] = true
	}

	requiredPermissions := []string{
		organizationAPIKeyPermissionFileRead,
		organizationAPIKeyPermissionFileCreate,
		organizationAPIKeyPermissionFileUpdate,
		organizationAPIKeyPermissionFileDelete,
	}
	for _, permissionName := range requiredPermissions {
		if !permissionNames[permissionName] {
			t.Fatalf("expected default product resource permission %q to exist", permissionName)
		}
	}

	return organizationAPIKeyResourcePermissions{
		FileRead:   organizationAPIKeyPermissionFileRead,
		FileCreate: organizationAPIKeyPermissionFileCreate,
		FileUpdate: organizationAPIKeyPermissionFileUpdate,
		FileDelete: organizationAPIKeyPermissionFileDelete,
	}
}

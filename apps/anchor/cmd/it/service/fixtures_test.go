package service_test

import (
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/permission"
	"anchor/internal/domain/product"
	"anchor/internal/domain/product/apikey"
	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/domain/tenant"
	"anchor/internal/service"
)

type PlatformTenantTest struct {
	tenantID string                //nolint:unused // May be used in future tests
	users    []PlatformUserTest    //nolint:unused // May be used in future tests
	products []PlatformProductTest //nolint:unused // May be used in future tests
}

type PlatformProductTest struct {
	productID string //nolint:unused // May be used in future tests
}

type PlatformUserTest struct {
	userID string //nolint:unused // May be used in future tests
}

func GivenARandomTenant(t *testing.T) tenant.PlatformTenant {
	tenantCreated, err := TenantRepository.Create(
		t.Context(), tenant.PlatformTenant{
			ID:     ids.MustNew("tenant"),
			Name:   Faker.RandomStringWithLength(20),
			Status: tenant.Active,
		},
	)
	require.NoError(t, err, "Failed to create random tenant")
	return tenantCreated
}

func GivenARandomProduct(t *testing.T, tenantID string) product.Product {
	product := product.Product{
		ID:               ids.MustNew("product"),
		PlatformTenantID: tenantID,
		Name:             Faker.RandomStringWithLength(20),
	}
	productCreated, err := ProductRepository.Create(
		t.Context(), product,
	)
	require.NoError(t, err, "Failed to create random product")
	return productCreated
}

type TenantAndProduct struct {
	Tenant  tenant.PlatformTenant
	Product product.Product
}

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

func GivenATenantAndProduct(t *testing.T) TenantAndProduct {
	randomTenant := GivenARandomTenant(t)
	return TenantAndProduct{
		Tenant:  randomTenant,
		Product: GivenARandomProduct(t, randomTenant.ID),
	}
}

func GivenBasicAnchorPermissions(t *testing.T, productID string) []string {
	permissions := slicex.Map(
		service.GeneratePermissions(), func(t permission.ProductPermission) string {
			return t.Name
		},
	)
	for _, perm := range permissions {
		_, err := PermissionRepository.Create(
			t.Context(), permission.ProductPermission{
				ProductID:   productID,
				Name:        perm,
				Description: nil,
			},
		)
		require.NoError(t, err, "Failed to create permission '%s'", perm)
	}
	return permissions
}

func GivenBasicProductResourcePermissions(t *testing.T, productID string) []string {
	permissions := []string{
		organizationAPIKeyPermissionFileRead,
		organizationAPIKeyPermissionFileCreate,
		organizationAPIKeyPermissionFileUpdate,
		organizationAPIKeyPermissionFileDelete,
	}
	for _, perm := range permissions {
		_, err := ResourcePermissionRepo.Create(
			t.Context(), resourcepermission.ProductResourcePermission{
				ProductID:     productID,
				Name:          perm,
				Description:   ptr.Ptr("Test resource permission"),
				ScopeModifier: ptr.Ptr("GLOBAL"),
			},
		)
		require.NoError(t, err, "Failed to create resource permission '%s'", perm)
	}
	return permissions
}

func GivenOrganizationAPIKeyResourcePermissionSet() organizationAPIKeyResourcePermissions {
	return organizationAPIKeyResourcePermissions{
		FileRead:   organizationAPIKeyPermissionFileRead,
		FileCreate: organizationAPIKeyPermissionFileCreate,
		FileUpdate: organizationAPIKeyPermissionFileUpdate,
		FileDelete: organizationAPIKeyPermissionFileDelete,
	}
}

func GivenAPIKey(t *testing.T, productID string, permissions []string) (
	apikey.ProductAPIKey, string,
) {
	name := Faker.RandomStringWithLength(20)
	apiKey, value, err := APIKeyService.Create(
		t.Context(), apikey.CreateProductAPIKeyInput{
			ProductID:   productID,
			Name:        name,
			Description: nil,
			Permissions: permissions,
		},
	)
	require.NoError(t, err, "Failed to create API key")
	return apiKey, value
}

func GivenADeactivatedAPIKey(t *testing.T, productID string, permissions []string) (
	apikey.ProductAPIKey, string,
) {
	apiKey, value := GivenAPIKey(t, productID, permissions)
	apiKey.Status = apikey.StatusInactive
	updatedAPIKey, err := APIKeyRepository.Update(t.Context(), apiKey)
	require.NoError(t, err, "Failed to deactivate API key")
	return updatedAPIKey, value
}

package repository_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/product"
	"anchor/internal/domain/product/role"
	"anchor/internal/domain/product/user"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/domain/tenant"

	"github.com/stretchr/testify/require"
)

// fixtureCounter disambiguates fixtures created multiple times within the
// same test (t.Name() alone repeats, and several entities enforce a
// per-parent uniqueness constraint on name/email).
var fixtureCounter atomic.Int64

func uniqueName(t *testing.T, prefix string) string {
	t.Helper()
	return fmt.Sprintf("%s %s %d", prefix, t.Name(), fixtureCounter.Add(1))
}

// createTenant inserts a platform tenant fixture and returns its ID.
func createTenant(t *testing.T) string {
	t.Helper()

	entity := tenant.PlatformTenant{
		Name:   uniqueName(t, "Test Tenant"),
		Status: tenant.Active,
	}
	entity.GenerateID()

	created, err := repoCtx.TenantRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created.ID
}

// createProduct inserts a product fixture under tenantID and returns its ID.
func createProduct(t *testing.T, tenantID string) string {
	t.Helper()

	entity := product.Product{
		PlatformTenantID: tenantID,
		Name:             uniqueName(t, "Test Product"),
		Description:      "fixture",
		Config:           product.DefaultConfig(),
	}
	entity.GenerateID()

	created, err := repoCtx.ProductRepository.Create(context.Background(), entity)
	require.NoError(t, err)
	root := resourcetype.NewOrganization(created.ID)
	_, err = repoCtx.DB.ExecContext(context.Background(),
		"INSERT INTO product_resource_types (id, product_id, slug, name) VALUES ($1, $2, $3, $4)",
		root.ID, root.ProductID, root.Slug, root.Name)
	require.NoError(t, err)

	return created.ID
}

// organizationTypeID returns the ID of the organization type createProduct
// seeds, the scope every role and permission fixture holds.
func organizationTypeID(t *testing.T, productID string) string {
	t.Helper()

	var id string
	require.NoError(t, repoCtx.DB.QueryRowContext(context.Background(),
		"SELECT id FROM product_resource_types WHERE product_id = $1 AND slug = $2",
		productID, resourcetype.OrganizationSlug).Scan(&id))
	return id
}

// createOrganization inserts an organization fixture under productID and returns its ID.
func createOrganization(t *testing.T, productID string) string {
	t.Helper()

	entity := organization.Organization{
		ProductID: productID,
		Name:      uniqueName(t, "Test Organization"),
	}
	entity.GenerateID()

	created, err := repoCtx.OrganizationRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created.ID
}

// createProductRole inserts a product role fixture under productID and returns its ID.
func createProductRole(t *testing.T, productID string) string {
	t.Helper()

	entity := role.NewManaged(productID, uniqueName(t, "Test Role"), "fixture")
	entity.ResourceTypeID = organizationTypeID(t, productID)

	created, err := repoCtx.ProductRoleRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created.ID
}

// createProductUser inserts a product user fixture under productID and returns its ID.
func createProductUser(t *testing.T, productID string) string {
	t.Helper()

	entity := user.ProductUser{
		ProductID: productID,
		Email:     uniqueName(t, "user") + "@example.com",
		Name:      uniqueName(t, "Test User"),
		Status:    user.ProductUserStatusActive,
	}
	entity.GenerateID()

	created, err := repoCtx.ProductUserRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created.ID
}

// tenantProductOrgChain builds Tenant -> Product -> Organization and returns
// the product and organization IDs — the fixture chain every product- and
// organization-scoped search test needs. The tenant ID itself has no caller
// in this suite (every Search method scopes by product or organization, not
// tenant directly), so it isn't returned.
func tenantProductOrgChain(t *testing.T) (string, string) {
	t.Helper()

	tenantID := createTenant(t)
	productID := createProduct(t, tenantID)
	organizationID := createOrganization(t, productID)

	return productID, organizationID
}

// tenantProductChain builds Tenant -> Product and returns the product ID —
// for product-scoped search tests that don't need an organization. See
// tenantProductOrgChain for why the tenant ID isn't returned.
func tenantProductChain(t *testing.T) string {
	t.Helper()

	tenantID := createTenant(t)
	return createProduct(t, tenantID)
}

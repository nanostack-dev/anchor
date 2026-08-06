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
	"anchor/internal/domain/tenant"
	"anchor/internal/domain/workspace"

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

	return created.ID
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

// createWorkspace inserts a workspace fixture under organizationID and returns its ID.
func createWorkspace(t *testing.T, organizationID string) string {
	t.Helper()

	entity := workspace.Workspace{
		OrganizationID: organizationID,
		Name:           uniqueName(t, "Test Workspace"),
	}
	entity.GenerateID()

	created, err := repoCtx.WorkspaceRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created.ID
}

// createProductRole inserts a product role fixture under productID and returns its ID.
func createProductRole(t *testing.T, productID string) string {
	t.Helper()

	entity := role.ProductRole{
		ProductID:   productID,
		Name:        uniqueName(t, "Test Role"),
		Description: "fixture",
	}
	entity.GenerateID()

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

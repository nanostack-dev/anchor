package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/organization"
	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/domain/permission"
	"anchor/internal/domain/platform"
	"anchor/internal/domain/product"
	"anchor/internal/domain/product/apikey"
	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/domain/product/user"
	"anchor/internal/repository"
)

// The searches below run on the generated db/search stack. Each test states the
// expected page as plain SQL (the same WHERE and ORDER BY), compares it with the
// page the builder returns on the same rows, and checks that a sibling tenant or
// product never leaks in.

func idsOf[T any](items []T, id func(T) string) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = id(item)
	}
	return out
}

func TestOrganizationSearchOrdersAndScopesToTheProduct(t *testing.T) {
	productID, _ := tenantProductOrgChain(t)
	for range 4 {
		createOrganization(t, productID)
	}
	otherProduct, _ := tenantProductOrgChain(t)

	want := queryIDs(t,
		`SELECT id FROM organizations WHERE product_id = $1 ORDER BY created_at ASC, id ASC LIMIT 3`, productID)

	q := repository.NewOrganizationSearch(productID).Limit(3)
	result, err := repoCtx.OrganizationRepository.SearchByProductID(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, want, idsOf(result.Items, func(o organization.Organization) string { return o.ID }))
	assert.Equal(t, int64(5), result.Total)

	other, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), repository.NewOrganizationSearch(otherProduct))
	require.NoError(t, err)
	for _, item := range other.Items {
		assert.Equal(t, otherProduct, item.ProductID)
	}
}

func TestOrganizationSearchFullTextIsCaseInsensitiveLiteralAndBlankMatchesAll(t *testing.T) {
	productID, _ := tenantProductOrgChain(t)
	entity := organization.Organization{ProductID: productID, Name: "100% Renewable Atlas"}
	entity.GenerateID()
	_, err := repoCtx.OrganizationRepository.Create(context.Background(), entity)
	require.NoError(t, err)
	decoy := organization.Organization{ProductID: productID, Name: "1000 Renewable Atlas"}
	decoy.GenerateID()
	_, err = repoCtx.OrganizationRepository.Create(context.Background(), decoy)
	require.NoError(t, err)

	result, err := repoCtx.OrganizationRepository.SearchByProductID(context.Background(),
		repository.NewOrganizationSearch(productID).FullText("  100% renewable "))
	require.NoError(t, err)
	assert.Equal(t, []string{entity.ID}, idsOf(result.Items, func(o organization.Organization) string { return o.ID }))

	all, err := repoCtx.OrganizationRepository.SearchByProductID(context.Background(),
		repository.NewOrganizationSearch(productID).FullText("   "))
	require.NoError(t, err)
	assert.Equal(t, int64(3), all.Total, "a blank term filters nothing: the chain's own organization plus both")
}

func TestOrganizationSearchCursorWalkVisitsEveryRowOnce(t *testing.T) {
	productID, _ := tenantProductOrgChain(t)
	for range cursorFixtureRows {
		createOrganization(t, productID)
	}
	want := queryIDs(
		t,
		`SELECT id FROM organizations WHERE product_id = $1 ORDER BY created_at ASC, id ASC`,
		productID,
	)

	ids, pages := walkByCursor(t,
		func(p search.Pagination) (search.Result[organization.Organization], error) {
			q := repository.NewOrganizationSearch(productID).Limit(p.Limit).After(p.Cursor)
			return repoCtx.OrganizationRepository.SearchByProductID(context.Background(), q)
		},
		func(o organization.Organization) string { return o.ID })

	assert.Equal(t, want, ids)
	assert.Greater(t, pages, 1)
}

func TestProductSearchOrdersAndScopesToTheTenant(t *testing.T) {
	tenantID := createTenant(t)
	for range 4 {
		createProduct(t, tenantID)
	}
	otherTenant := createTenant(t)
	createProduct(t, otherTenant)

	want := queryIDs(t,
		`SELECT id FROM products WHERE platform_tenant_id = $1 ORDER BY created_at ASC, id ASC LIMIT 3`, tenantID)

	result, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), repository.NewProductSearch(tenantID).Limit(3))
	require.NoError(t, err)
	assert.Equal(t, want, idsOf(result.Items, func(p product.Product) string { return p.ID }))
	assert.Equal(t, int64(4), result.Total)
	for _, item := range result.Items {
		assert.Equal(t, tenantID, item.PlatformTenantID)
	}
}

func TestProductUserSearchOrdersFiltersAndScopes(t *testing.T) {
	productID := tenantProductChain(t)
	for range 4 {
		createProductUser(t, productID)
	}
	otherProduct := tenantProductChain(t)
	createProductUser(t, otherProduct)

	want := queryIDs(t,
		`SELECT id FROM product_users WHERE product_id = $1 ORDER BY name DESC NULLS LAST, id DESC LIMIT 3`, productID)

	q := repository.NewProductUserSearch(productID).
		SortBy(repository.ProductUserSortName, search.SortDescending).Limit(3)
	result, err := repoCtx.ProductUserRepository.SearchByProductID(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, want, idsOf(result.Items, func(u user.ProductUser) string { return u.ID }))

	q = repository.NewProductUserSearch(productID)
	q = q.Status().In(user.ProductUserStatusActive)
	active, err := repoCtx.ProductUserRepository.SearchByProductID(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, int64(4), active.Total)
	for _, item := range active.Items {
		assert.Equal(t, productID, item.ProductID)
	}
}

func TestOrganizationAPIKeySearchLastUsedBoundsAreExclusive(t *testing.T) {
	_, organizationID := tenantProductOrgChain(t)
	var keys []string
	base := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	for i := range 3 {
		id := createOrganizationAPIKey(t, organizationID)
		_, err := repoCtx.DB.ExecContext(
			context.Background(),
			`UPDATE organization_api_keys SET last_used_at = $1 WHERE id = $2`,
			base.Add(time.Duration(i)*time.Hour),
			id,
		)
		require.NoError(t, err)
		keys = append(keys, id)
	}

	q := repository.NewOrganizationAPIKeySearch(organizationID)
	q = q.LastUsedAt().After(base)
	q = q.LastUsedAt().Before(base.Add(2 * time.Hour))
	result, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, []string{keys[1]}, idsOf(result.Items, func(k orgapikey.OrganizationAPIKey) string { return k.ID }),
		"a key used exactly at either bound is outside the exclusive range")

	want := queryIDs(t, `SELECT id FROM organization_api_keys WHERE organization_id = $1 `+
		`AND last_used_at > $2 ORDER BY created_at ASC, id ASC`, organizationID, base)
	q = repository.NewOrganizationAPIKeySearch(organizationID)
	q = q.LastUsedAt().After(base)
	later, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, want, idsOf(later.Items, func(k orgapikey.OrganizationAPIKey) string { return k.ID }))
}

func TestOrganizationAPIKeySearchScopesToTheOrganization(t *testing.T) {
	_, organizationID := tenantProductOrgChain(t)
	_, otherOrganization := tenantProductOrgChain(t)
	mine := createOrganizationAPIKey(t, organizationID)
	createOrganizationAPIKey(t, otherOrganization)

	result, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(), repository.NewOrganizationAPIKeySearch(organizationID))
	require.NoError(t, err)
	assert.Equal(t, []string{mine}, idsOf(result.Items, func(k orgapikey.OrganizationAPIKey) string { return k.ID }))
}

func TestProductAPIKeySearchOrdersAndScopesToTheProduct(t *testing.T) {
	productID := tenantProductChain(t)
	for range 4 {
		createProductAPIKey(t, productID, nil)
	}
	otherProduct := tenantProductChain(t)
	createProductAPIKey(t, otherProduct, nil)

	want := queryIDs(t,
		`SELECT id FROM product_api_keys WHERE product_id = $1 ORDER BY created_at ASC, id ASC LIMIT 3`, productID)
	result, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(), repository.NewProductAPIKeySearch(productID).Limit(3))
	require.NoError(t, err)
	assert.Equal(t, want, idsOf(result.Items, func(k apikey.ProductAPIKey) string { return k.ID }))
	assert.Equal(t, int64(4), result.Total)
}

func TestPlatformUserSearchOrdersFoldsTheRoleAndScopesToTheTenant(t *testing.T) {
	tenantID := createTenant(t)
	for range 3 {
		createPlatformUser(t, tenantID)
	}
	otherTenant := createTenant(t)
	createPlatformUser(t, otherTenant)

	want := queryIDs(t,
		`SELECT id FROM platform_users WHERE platform_tenant_id = $1 ORDER BY created_at ASC, id ASC`, tenantID)
	result, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), repository.NewPlatformUserSearch(tenantID))
	require.NoError(t, err)
	assert.Equal(t, want, idsOf(result.Items, func(u platform.User) string { return u.ID }))

	q := repository.NewPlatformUserSearch(tenantID)
	q = q.Role().In("ADMIN")
	byRole, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, int64(3), byRole.Total, "the role filter ignores case")
}

func TestOrganizationMemberSearchOrdersAndScopesToTheOrganization(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)
	_, otherOrganization := tenantProductOrgChain(t)
	var members []string
	for range 3 {
		userID := createOrgMembership(t, productID, organizationID)
		members = append(members, userID)
	}
	otherProduct := tenantProductChain(t)
	createOrgMembership(t, otherProduct, createOrganization(t, otherProduct))
	_ = otherOrganization

	want := queryIDs(t, `SELECT m.product_user_id FROM organization_memberships m `+
		`JOIN organizations o ON o.id = m.organization_id `+
		`WHERE o.product_id = $1 AND m.organization_id = $2`,
		productID, organizationID)
	q := repository.NewOrganizationMemberSearch(
		repository.OrganizationMemberScope{ProductID: productID, OrganizationID: organizationID})
	result, err := repoCtx.OrganizationMembershipRepository.SearchByOrgID(context.Background(), q)
	require.NoError(t, err)
	found := idsOf(result.Items, func(m organization.Membership) string { return m.ProductUserID })
	assert.ElementsMatch(t, want, found)
	assert.ElementsMatch(t, members, found)
	assert.Equal(t, int64(3), result.Total)
}

func TestProductPermissionSearchFoldsNamesAndScopesToTheProduct(t *testing.T) {
	productID := tenantProductChain(t)
	otherProduct := tenantProductChain(t)
	mine := createProductPermission(t, productID, "Document:Read", nil)
	createProductPermission(t, productID, "Document:Write", nil)
	createProductPermission(t, otherProduct, "Document:Read", nil)

	q := repository.NewProductPermissionSearch(productID)
	q = q.Name().In("document:read")
	result, err := repoCtx.ProductPermissionRepository.SearchByProduct(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{mine.Name},
		idsOf(result.Items, func(p permission.ProductPermission) string { return p.Name }),
	)
}

func TestProductResourcePermissionSearchFoldsNamesAndScopesToTheProduct(t *testing.T) {
	productID := tenantProductChain(t)
	otherProduct := tenantProductChain(t)
	mine := createResourcePermission(t, productID, "Document:Read")
	createResourcePermission(t, productID, "Document:Write")
	createResourcePermission(t, otherProduct, "Document:Read")

	q := repository.NewProductResourcePermissionSearch(productID)
	q = q.Name().In("DOCUMENT:READ")
	result, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, []string{mine.Name},
		idsOf(result.Items, func(p resourcepermission.ProductResourcePermission) string { return p.Name }))
}

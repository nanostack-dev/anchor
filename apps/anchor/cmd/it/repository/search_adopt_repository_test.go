package repository_test

import (
	"context"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/product"
	"anchor/internal/domain/product/user"
	"anchor/internal/repository"
)

// The searches below run on the generated db/search stack. Each test compares
// the page the builder returns with the page the SQL the hand-written query
// produced before (the oracle: the same WHERE and ORDER BY, written as SQL) on
// the same rows, then checks that a sibling tenant or product never leaks in.

func idsOf[T any](items []T, id func(T) string) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = id(item)
	}
	return out
}

func TestOrganizationSearchMatchesTheLegacyOrderAndScopesToTheProduct(t *testing.T) {
	productID, _ := tenantProductOrgChain(t)
	for range 4 {
		createOrganization(t, productID)
	}
	otherProduct, _ := tenantProductOrgChain(t)

	oracle := queryIDs(t,
		`SELECT id FROM organizations WHERE product_id = $1 ORDER BY created_at ASC, id ASC LIMIT 3`, productID)

	q := repository.NewOrganizationSearch(productID).Limit(3)
	result, err := repoCtx.OrganizationRepository.SearchByProductID(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, oracle, idsOf(result.Items, func(o organization.Organization) string { return o.ID }))
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
	oracle := queryIDs(t, `SELECT id FROM organizations WHERE product_id = $1 ORDER BY created_at ASC, id ASC`, productID)

	ids, pages := walkByCursor(t,
		func(p search.Pagination) (search.Result[organization.Organization], error) {
			q := repository.NewOrganizationSearch(productID).Limit(int32(p.Limit)).After(p.Cursor)
			return repoCtx.OrganizationRepository.SearchByProductID(context.Background(), q)
		},
		func(o organization.Organization) string { return o.ID })

	assert.Equal(t, oracle, ids)
	assert.Greater(t, pages, 1)
}

func TestProductSearchMatchesTheLegacyOrderAndScopesToTheTenant(t *testing.T) {
	tenantID := createTenant(t)
	for range 4 {
		createProduct(t, tenantID)
	}
	otherTenant := createTenant(t)
	createProduct(t, otherTenant)

	oracle := queryIDs(t,
		`SELECT id FROM products WHERE platform_tenant_id = $1 ORDER BY created_at ASC, id ASC LIMIT 3`, tenantID)

	result, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), repository.NewProductSearch(tenantID).Limit(3))
	require.NoError(t, err)
	assert.Equal(t, oracle, idsOf(result.Items, func(p product.Product) string { return p.ID }))
	assert.Equal(t, int64(4), result.Total)
	for _, item := range result.Items {
		assert.Equal(t, tenantID, item.PlatformTenantID)
	}
}

func TestProductUserSearchMatchesTheLegacyOrderFilterAndScope(t *testing.T) {
	productID := tenantProductChain(t)
	for range 4 {
		createProductUser(t, productID)
	}
	otherProduct := tenantProductChain(t)
	createProductUser(t, otherProduct)

	oracle := queryIDs(t,
		`SELECT id FROM product_users WHERE product_id = $1 ORDER BY name DESC NULLS LAST, id DESC LIMIT 3`, productID)

	q := repository.NewProductUserSearch(productID).
		SortBy(repository.ProductUserSortName, search.SortDescending).Limit(3)
	result, err := repoCtx.ProductUserRepository.SearchByProductID(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, oracle, idsOf(result.Items, func(u user.ProductUser) string { return u.ID }))

	q = repository.NewProductUserSearch(productID)
	q = q.Status().In(user.ProductUserStatusActive)
	active, err := repoCtx.ProductUserRepository.SearchByProductID(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, int64(4), active.Total)
	for _, item := range active.Items {
		assert.Equal(t, productID, item.ProductID)
	}
}

package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/product"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductRepositorySearchByTenantIDEmpty(t *testing.T) {
	tenantID := createTenant(t)

	result, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestProductRepositorySearchByTenantIDPaginationBoundaries(t *testing.T) {
	tenantID := createTenant(t)

	const total = 5
	created := make([]string, 0, total)
	for range total {
		created = append(created, createProduct(t, tenantID))
	}

	// Full page in one shot.
	result, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Sort: []search.Sort[product.SortFieldProduct]{
				{Field: product.SortFieldProductCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), result.Total)
	assert.Equal(t, total, result.Count)
	require.Len(t, result.Items, total)
	for i, item := range result.Items {
		assert.Equal(t, created[i], item.ID)
	}

	// First page of 2: total still reflects all matches, not the page.
	page1, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Sort: []search.Sort[product.SortFieldProduct]{
				{Field: product.SortFieldProductCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 2, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page1.Total)
	assert.Equal(t, 2, page1.Count)
	require.Len(t, page1.Items, 2)
	assert.Equal(t, created[0], page1.Items[0].ID)
	assert.Equal(t, created[1], page1.Items[1].ID)

	// Second page of 2.
	page2, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Sort: []search.Sort[product.SortFieldProduct]{
				{Field: product.SortFieldProductCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 2, Offset: 2},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page2.Total)
	assert.Equal(t, 2, page2.Count)
	require.Len(t, page2.Items, 2)
	assert.Equal(t, created[2], page2.Items[0].ID)
	assert.Equal(t, created[3], page2.Items[1].ID)

	// Last page: partial, offset past most rows.
	page3, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Sort: []search.Sort[product.SortFieldProduct]{
				{Field: product.SortFieldProductCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 2, Offset: 4},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page3.Total)
	assert.Equal(t, 1, page3.Count)
	require.Len(t, page3.Items, 1)
	assert.Equal(t, created[4], page3.Items[0].ID)

	// Offset beyond total: no rows, total still accurate.
	pageOOB, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Pagination: search.Pagination{Limit: 10, Offset: 50},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestProductRepositorySearchByTenantIDSortDirections(t *testing.T) {
	tenantID := createTenant(t)

	// Names chosen so ascending/descending name order is unambiguous.
	names := []string{"Alpha", "Bravo", "Charlie"}
	ids := make(map[string]string, len(names))
	for _, name := range names {
		entity := product.Product{
			PlatformTenantID: tenantID,
			Name:             name,
			Config:           product.DefaultConfig(),
		}
		entity.GenerateID()
		created, err := repoCtx.ProductRepository.Create(context.Background(), entity)
		require.NoError(t, err)
		ids[name] = created.ID
	}

	for _, field := range []product.SortFieldProduct{
		product.SortFieldProductCreatedAt,
		product.SortFieldProductUpdatedAt,
		product.SortFieldProductName,
	} {
		asc, err := repoCtx.ProductRepository.SearchByTenantID(
			context.Background(), tenantID,
			search.Request[product.SearchProductFilter, product.SortFieldProduct]{
				Sort: []search.Sort[product.SortFieldProduct]{
					{Field: field, Direction: search.SortAscending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		)
		require.NoError(t, err)
		require.Len(t, asc.Items, 3)

		desc, err := repoCtx.ProductRepository.SearchByTenantID(
			context.Background(), tenantID,
			search.Request[product.SearchProductFilter, product.SortFieldProduct]{
				Sort: []search.Sort[product.SortFieldProduct]{
					{Field: field, Direction: search.SortDescending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		)
		require.NoError(t, err)
		require.Len(t, desc.Items, 3)

		// Descending must be the exact reverse of ascending for every field.
		for i := range asc.Items {
			assert.Equal(t, asc.Items[i].ID, desc.Items[len(desc.Items)-1-i].ID)
		}
	}

	// Name sort has an unambiguous, independently known order.
	ascByName, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Sort: []search.Sort[product.SortFieldProduct]{
				{Field: product.SortFieldProductName, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, ascByName.Items, 3)
	assert.Equal(t, []string{ids["Alpha"], ids["Bravo"], ids["Charlie"]},
		[]string{ascByName.Items[0].ID, ascByName.Items[1].ID, ascByName.Items[2].ID})

	descByName, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Sort: []search.Sort[product.SortFieldProduct]{
				{Field: product.SortFieldProductName, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, descByName.Items, 3)
	assert.Equal(t, []string{ids["Charlie"], ids["Bravo"], ids["Alpha"]},
		[]string{descByName.Items[0].ID, descByName.Items[1].ID, descByName.Items[2].ID})
}

func TestProductRepositorySearchByTenantIDFilterByIDsAndNames(t *testing.T) {
	tenantID := createTenant(t)

	first := createProduct(t, tenantID)
	second := createProduct(t, tenantID)
	_ = createProduct(t, tenantID) // unrelated third product, must not match

	byID, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Filter:     &product.SearchProductFilter{IDs: []string{first, second}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byID.Total)
	gotIDs := []string{byID.Items[0].ID, byID.Items[1].ID}
	assert.ElementsMatch(t, []string{first, second}, gotIDs)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Filter:     &product.SearchProductFilter{IDs: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestProductRepositorySearchByTenantIDFilterByNames(t *testing.T) {
	tenantID := createTenant(t)

	name := uniqueName(t, "Named Product")
	entity := product.Product{
		PlatformTenantID: tenantID,
		Name:             name,
		Config:           product.DefaultConfig(),
	}
	entity.GenerateID()
	matching, err := repoCtx.ProductRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	_ = createProduct(t, tenantID) // unrelated product, must not match

	byName, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Filter:     &product.SearchProductFilter{Names: []string{name}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, byName.Items, 1)
	assert.Equal(t, matching.ID, byName.Items[0].ID)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Filter:     &product.SearchProductFilter{Names: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestProductRepositorySearchByTenantIDFullTextSearch(t *testing.T) {
	tenantID := createTenant(t)

	entity := product.Product{
		PlatformTenantID: tenantID,
		Name:             "Atlas Migration Product",
		Config:           product.DefaultConfig(),
	}
	entity.GenerateID()
	matching, err := repoCtx.ProductRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	other := product.Product{
		PlatformTenantID: tenantID,
		Name:             "Billing Console",
		Config:           product.DefaultConfig(),
	}
	other.GenerateID()
	_, err = repoCtx.ProductRepository.Create(context.Background(), other)
	require.NoError(t, err)

	// The filter is a plain SQL LIKE, not tsvector — it is case-sensitive, so
	// the term must match the stored case.
	term := "Atlas"
	result, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			FullTextSearch: &term,
			Pagination:     search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, matching.ID, result.Items[0].ID)
}

func TestProductRepositorySearchByTenantIDScopesToTenant(t *testing.T) {
	tenantID := createTenant(t)
	otherTenantID := createTenant(t)

	inScope := createProduct(t, tenantID)
	_ = createProduct(t, otherTenantID) // must never leak into the first tenant's results

	result, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, inScope, result.Items[0].ID)

	otherResult, err := repoCtx.ProductRepository.SearchByTenantID(
		context.Background(), otherTenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
}

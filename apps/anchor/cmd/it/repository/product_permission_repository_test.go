package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/permission"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createProductPermission inserts a product permission fixture under productID
// and returns the created domain value. Unlike the other fixtures in
// fixtures_test.go, ProductPermission has no shared helper: it is keyed by
// (product_id, name) instead of a generated ID, so the caller supplies the name.
func createProductPermission(
	t *testing.T, productID, name string, description *string,
) permission.ProductPermission {
	t.Helper()

	entity := permission.ProductPermission{
		ProductID:   productID,
		Name:        name,
		Description: description,
	}

	created, err := repoCtx.ProductPermissionRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created
}

func TestProductPermissionRepositorySearchByProductEmpty(t *testing.T) {
	productID := tenantProductChain(t)

	result, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestProductPermissionRepositorySearchByProductPaginationBoundaries(t *testing.T) {
	productID := tenantProductChain(t)

	const total = 5
	created := make([]string, 0, total)
	for range total {
		perm := createProductPermission(t, productID, uniqueName(t, "perm"), nil)
		created = append(created, perm.Name)
	}

	// Full page in one shot.
	result, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Sort: []search.Sort[permission.SortFieldProductPermission]{
				{Field: permission.SortFieldProductPermissionCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), result.Total)
	assert.Equal(t, total, result.Count)
	require.Len(t, result.Items, total)
	for i, item := range result.Items {
		assert.Equal(t, created[i], item.Name)
	}

	// First page of 2: total still reflects all matches, not the page.
	page1, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Sort: []search.Sort[permission.SortFieldProductPermission]{
				{Field: permission.SortFieldProductPermissionCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 2, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page1.Total)
	assert.Equal(t, 2, page1.Count)
	require.Len(t, page1.Items, 2)
	assert.Equal(t, created[0], page1.Items[0].Name)
	assert.Equal(t, created[1], page1.Items[1].Name)

	// Second page of 2.
	page2, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Sort: []search.Sort[permission.SortFieldProductPermission]{
				{Field: permission.SortFieldProductPermissionCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 2, Offset: 2},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page2.Total)
	assert.Equal(t, 2, page2.Count)
	require.Len(t, page2.Items, 2)
	assert.Equal(t, created[2], page2.Items[0].Name)
	assert.Equal(t, created[3], page2.Items[1].Name)

	// Last page: partial, offset past most rows.
	page3, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Sort: []search.Sort[permission.SortFieldProductPermission]{
				{Field: permission.SortFieldProductPermissionCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 2, Offset: 4},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page3.Total)
	assert.Equal(t, 1, page3.Count)
	require.Len(t, page3.Items, 1)
	assert.Equal(t, created[4], page3.Items[0].Name)

	// Offset beyond total: no rows, total still accurate.
	pageOOB, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Pagination: search.Pagination{Limit: 10, Offset: 50},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestProductPermissionRepositorySearchByProductSortDirections(t *testing.T) {
	productID := tenantProductChain(t)

	// Names chosen so ascending/descending name order is unambiguous.
	names := []string{"alpha:read", "bravo:read", "charlie:read"}
	for _, name := range names {
		createProductPermission(t, productID, name, nil)
	}

	// Sort by name.
	ascName, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Sort: []search.Sort[permission.SortFieldProductPermission]{
				{Field: permission.SortFieldProductPermissionName, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, ascName.Items, 3)
	assert.Equal(t, []string{"alpha:read", "bravo:read", "charlie:read"},
		[]string{ascName.Items[0].Name, ascName.Items[1].Name, ascName.Items[2].Name})

	descName, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Sort: []search.Sort[permission.SortFieldProductPermission]{
				{Field: permission.SortFieldProductPermissionName, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, descName.Items, 3)
	assert.Equal(t, []string{"charlie:read", "bravo:read", "alpha:read"},
		[]string{descName.Items[0].Name, descName.Items[1].Name, descName.Items[2].Name})

	// Sort by created_at.
	ascCreated, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Sort: []search.Sort[permission.SortFieldProductPermission]{
				{Field: permission.SortFieldProductPermissionCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, ascCreated.Items, 3)
	assert.Equal(t, []string{"alpha:read", "bravo:read", "charlie:read"},
		[]string{ascCreated.Items[0].Name, ascCreated.Items[1].Name, ascCreated.Items[2].Name})

	descCreated, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Sort: []search.Sort[permission.SortFieldProductPermission]{
				{Field: permission.SortFieldProductPermissionCreatedAt, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, descCreated.Items, 3)
	assert.Equal(t, []string{"charlie:read", "bravo:read", "alpha:read"},
		[]string{descCreated.Items[0].Name, descCreated.Items[1].Name, descCreated.Items[2].Name})

	// Sort by updated_at: update "alpha:read" so it becomes the most recently updated.
	toUpdate := repoCtx.ProductPermissionRepository.FindByProductIDAndPermissionName(
		context.Background(), productID, "alpha:read",
	)
	require.NoError(t, toUpdate.Err())
	require.True(t, toUpdate.IsPresent())
	_, err = repoCtx.ProductPermissionRepository.Update(context.Background(), toUpdate.Value())
	require.NoError(t, err)

	descUpdated, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Sort: []search.Sort[permission.SortFieldProductPermission]{
				{Field: permission.SortFieldProductPermissionUpdatedAt, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, descUpdated.Items, 3)
	assert.Equal(t, "alpha:read", descUpdated.Items[0].Name)

	ascUpdated, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Sort: []search.Sort[permission.SortFieldProductPermission]{
				{Field: permission.SortFieldProductPermissionUpdatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, ascUpdated.Items, 3)
	assert.Equal(t, "alpha:read", ascUpdated.Items[len(ascUpdated.Items)-1].Name)
}

func TestProductPermissionRepositorySearchByProductFilterByNames(t *testing.T) {
	productID := tenantProductChain(t)

	first := createProductPermission(t, productID, uniqueName(t, "perm:first"), nil)
	second := createProductPermission(t, productID, uniqueName(t, "perm:second"), nil)
	_ = createProductPermission(t, productID, uniqueName(t, "perm:third"), nil) // unrelated, must not match

	byNames, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Filter:     &permission.SearchProductPermissionFilter{Names: []string{first.Name, second.Name}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byNames.Total)
	gotNames := []string{byNames.Items[0].Name, byNames.Items[1].Name}
	assert.ElementsMatch(t, []string{first.Name, second.Name}, gotNames)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Filter:     &permission.SearchProductPermissionFilter{Names: []string{"does-not-exist:action"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestProductPermissionRepositorySearchByProductFullTextSearch(t *testing.T) {
	productID := tenantProductChain(t)

	description := "Grants read access to Atlas resources"
	matching := createProductPermission(t, productID, uniqueName(t, "Atlas:read"), &description)

	otherDescription := "Grants write access to billing resources"
	_ = createProductPermission(t, productID, uniqueName(t, "Billing:write"), &otherDescription)

	// The filter is a plain SQL LIKE, not tsvector — it is case-sensitive, so
	// the term must match the stored case exactly.
	term := "Atlas"
	result, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			FullTextSearch: &term,
			Pagination:     search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, matching.Name, result.Items[0].Name)

	// Full-text search also matches on description.
	descTerm := "billing resources"
	descResult, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			FullTextSearch: &descTerm,
			Pagination:     search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, descResult.Items, 1)
	assert.Equal(t, otherDescription, *descResult.Items[0].Description)
}

func TestProductPermissionRepositorySearchByProductScopesToProduct(t *testing.T) {
	productID := tenantProductChain(t)
	otherProductID := tenantProductChain(t)

	inScope := createProductPermission(t, productID, uniqueName(t, "perm"), nil)
	_ = createProductPermission(
		t,
		otherProductID,
		uniqueName(t, "perm"),
		nil,
	) // must never leak into the first product's results

	result, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, inScope.Name, result.Items[0].Name)

	otherResult, err := repoCtx.ProductPermissionRepository.SearchByProduct(
		context.Background(), otherProductID,
		search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
}

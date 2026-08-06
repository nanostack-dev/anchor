package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	resourcepermission "anchor/internal/domain/product/resource_permission"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createResourcePermission inserts a resource permission fixture under
// productID with the given name (no description/scope modifier) and returns
// the created domain value.
func createResourcePermission(
	t *testing.T, productID, name string,
) resourcepermission.ProductResourcePermission {
	t.Helper()
	return createResourcePermissionFull(t, productID, name, nil, nil)
}

// createResourcePermissionFull inserts a resource permission fixture with an
// optional description and scope modifier.
func createResourcePermissionFull(
	t *testing.T, productID, name string, description, scopeModifier *string,
) resourcepermission.ProductResourcePermission {
	t.Helper()

	perm := resourcepermission.ProductResourcePermission{
		ProductID:     productID,
		Name:          name,
		Description:   description,
		ScopeModifier: scopeModifier,
	}

	created, err := repoCtx.ProductResourcePermissionRepository.Create(context.Background(), perm)
	require.NoError(t, err)

	return created
}

// backdateResourcePermission overwrites created_at/updated_at directly in
// the DB. Create() lets Postgres default these columns, so ordering tests
// need explicit, well-separated timestamps to be deterministic.
func backdateResourcePermission(
	t *testing.T, productID, name string, createdAt, updatedAt time.Time,
) {
	t.Helper()

	_, err := repoCtx.DB.Exec(
		`UPDATE product_resource_permissions SET created_at = $1, updated_at = $2 WHERE product_id = $3 AND name = $4`,
		createdAt, updatedAt, productID, name,
	)
	require.NoError(t, err)
}

func TestProductResourcePermissionRepositorySearchByProductEmpty(t *testing.T) {
	productID := tenantProductChain(t)

	result, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestProductResourcePermissionRepositorySearchByProductPaginationBoundaries(t *testing.T) {
	productID := tenantProductChain(t)

	const total = 5
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	names := make([]string, 0, total)
	for i := range total {
		name := uniqueName(t, "Permission")
		createResourcePermission(t, productID, name)
		backdateResourcePermission(t, productID, name, base.Add(time.Duration(i)*time.Second), base)
		names = append(names, name)
	}

	sortByCreatedAt := []search.Sort[resourcepermission.SortFieldProductResourcePermission]{
		{Field: resourcepermission.SortFieldProductResourcePermissionCreatedAt, Direction: search.SortAscending},
	}

	// Full page in one shot.
	result, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Sort:       sortByCreatedAt,
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), result.Total)
	assert.Equal(t, total, result.Count)
	require.Len(t, result.Items, total)
	for i, item := range result.Items {
		assert.Equal(t, names[i], item.Name)
	}

	// First page of 2.
	page1, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Sort:       sortByCreatedAt,
			Pagination: search.Pagination{Limit: 2, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page1.Total)
	assert.Equal(t, 2, page1.Count)
	require.Len(t, page1.Items, 2)
	assert.Equal(t, names[0], page1.Items[0].Name)
	assert.Equal(t, names[1], page1.Items[1].Name)

	// Second page of 2.
	page2, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Sort:       sortByCreatedAt,
			Pagination: search.Pagination{Limit: 2, Offset: 2},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page2.Total)
	assert.Equal(t, 2, page2.Count)
	require.Len(t, page2.Items, 2)
	assert.Equal(t, names[2], page2.Items[0].Name)
	assert.Equal(t, names[3], page2.Items[1].Name)

	// Third page: partial, only one row left.
	page3, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Sort:       sortByCreatedAt,
			Pagination: search.Pagination{Limit: 2, Offset: 4},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page3.Total)
	assert.Equal(t, 1, page3.Count)
	require.Len(t, page3.Items, 1)
	assert.Equal(t, names[4], page3.Items[0].Name)

	// Offset beyond total: no rows, total still accurate.
	pageOOB, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Pagination: search.Pagination{Limit: 10, Offset: 50},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestProductResourcePermissionRepositorySearchByProductSortByName(t *testing.T) {
	productID := tenantProductChain(t)

	// Names chosen so ascending/descending order is unambiguous.
	names := []string{"Alpha", "Bravo", "Charlie"}
	for _, name := range names {
		createResourcePermission(t, productID, name)
	}

	asc, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Sort: []search.Sort[resourcepermission.SortFieldProductResourcePermission]{
				{Field: resourcepermission.SortFieldProductResourcePermissionName, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, asc.Items, 3)
	assert.Equal(t, []string{"Alpha", "Bravo", "Charlie"},
		[]string{asc.Items[0].Name, asc.Items[1].Name, asc.Items[2].Name})

	desc, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Sort: []search.Sort[resourcepermission.SortFieldProductResourcePermission]{
				{Field: resourcepermission.SortFieldProductResourcePermissionName, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, desc.Items, 3)
	assert.Equal(t, []string{"Charlie", "Bravo", "Alpha"},
		[]string{desc.Items[0].Name, desc.Items[1].Name, desc.Items[2].Name})
}

func TestProductResourcePermissionRepositorySearchByProductSortByCreatedAt(t *testing.T) {
	productID := tenantProductChain(t)

	base := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	names := make([]string, 0, 3)
	for i := range 3 {
		name := uniqueName(t, "Permission")
		createResourcePermission(t, productID, name)
		backdateResourcePermission(t, productID, name, base.Add(time.Duration(i)*time.Hour), base)
		names = append(names, name)
	}

	asc, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Sort: []search.Sort[resourcepermission.SortFieldProductResourcePermission]{
				{
					Field:     resourcepermission.SortFieldProductResourcePermissionCreatedAt,
					Direction: search.SortAscending,
				},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, asc.Items, 3)
	assert.Equal(t, []string{asc.Items[0].Name, asc.Items[1].Name, asc.Items[2].Name}, names)

	desc, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Sort: []search.Sort[resourcepermission.SortFieldProductResourcePermission]{
				{
					Field:     resourcepermission.SortFieldProductResourcePermissionCreatedAt,
					Direction: search.SortDescending,
				},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, desc.Items, 3)
	assert.Equal(t, []string{names[2], names[1], names[0]},
		[]string{desc.Items[0].Name, desc.Items[1].Name, desc.Items[2].Name})
}

func TestProductResourcePermissionRepositorySearchByProductSortByUpdatedAt(t *testing.T) {
	productID := tenantProductChain(t)

	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	names := make([]string, 0, 3)
	for i := range 3 {
		name := uniqueName(t, "Permission")
		createResourcePermission(t, productID, name)
		backdateResourcePermission(t, productID, name, base, base.Add(time.Duration(i)*time.Hour))
		names = append(names, name)
	}

	asc, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Sort: []search.Sort[resourcepermission.SortFieldProductResourcePermission]{
				{
					Field:     resourcepermission.SortFieldProductResourcePermissionUpdatedAt,
					Direction: search.SortAscending,
				},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, asc.Items, 3)
	assert.Equal(t, []string{asc.Items[0].Name, asc.Items[1].Name, asc.Items[2].Name}, names)

	desc, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Sort: []search.Sort[resourcepermission.SortFieldProductResourcePermission]{
				{
					Field:     resourcepermission.SortFieldProductResourcePermissionUpdatedAt,
					Direction: search.SortDescending,
				},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, desc.Items, 3)
	assert.Equal(t, []string{names[2], names[1], names[0]},
		[]string{desc.Items[0].Name, desc.Items[1].Name, desc.Items[2].Name})
}

func TestProductResourcePermissionRepositorySearchByProductFilterByNames(t *testing.T) {
	productID := tenantProductChain(t)

	first := createResourcePermission(t, productID, uniqueName(t, "FileRead"))
	second := createResourcePermission(t, productID, uniqueName(t, "FileWrite"))
	_ = createResourcePermission(t, productID, uniqueName(t, "Unrelated")) // must not match

	byNames, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Filter: &resourcepermission.SearchProductResourcePermissionFilter{
				Names: []string{first.Name, second.Name},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byNames.Total)
	gotNames := []string{byNames.Items[0].Name, byNames.Items[1].Name}
	assert.ElementsMatch(t, []string{first.Name, second.Name}, gotNames)

	// The filter lower-cases both sides, so it must also match on differing case.
	byNamesCaseInsensitive, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Filter: &resourcepermission.SearchProductResourcePermissionFilter{
				Names: []string{strings.ToUpper(first.Name)},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, byNamesCaseInsensitive.Items, 1)
	assert.Equal(t, first.Name, byNamesCaseInsensitive.Items[0].Name)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Filter: &resourcepermission.SearchProductResourcePermissionFilter{
				Names: []string{"does-not-exist"},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestProductResourcePermissionRepositorySearchByProductFilterByScopeModifiers(t *testing.T) {
	productID := tenantProductChain(t)

	read := createResourcePermissionFull(t, productID, uniqueName(t, "ReadPermission"), nil, new("read"))
	_ = createResourcePermissionFull(t, productID, uniqueName(t, "WritePermission"), nil, new("write"))
	_ = createResourcePermission(t, productID, uniqueName(t, "NoScopePermission")) // nil scope modifier

	result, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Filter: &resourcepermission.SearchProductResourcePermissionFilter{
				ScopeModifiers: []string{"read"},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, read.Name, result.Items[0].Name)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Filter: &resourcepermission.SearchProductResourcePermissionFilter{
				ScopeModifiers: []string{"does-not-exist"},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestProductResourcePermissionRepositorySearchByProductScopesToProduct(t *testing.T) {
	productID := tenantProductChain(t)
	otherProductID := tenantProductChain(t)

	inScope := createResourcePermission(t, productID, uniqueName(t, "InScope"))
	_ = createResourcePermission(t, otherProductID, uniqueName(t, "OtherScope")) // must never leak

	result, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), productID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, inScope.Name, result.Items[0].Name)

	otherResult, err := repoCtx.ProductResourcePermissionRepository.SearchByProduct(
		context.Background(), otherProductID,
		search.Request[
			resourcepermission.SearchProductResourcePermissionFilter,
			resourcepermission.SortFieldProductResourcePermission,
		]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
}

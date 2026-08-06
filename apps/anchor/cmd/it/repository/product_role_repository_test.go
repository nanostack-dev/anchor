package repository_test

import (
	"context"
	"testing"

	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/domain/product/role"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductRoleRepositorySearchByProductIDEmpty(t *testing.T) {
	productID := tenantProductChain(t)

	result, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestProductRoleRepositorySearchByProductIDPaginationBoundaries(t *testing.T) {
	productID := tenantProductChain(t)

	const total = 5
	created := make([]string, 0, total)
	for range total {
		created = append(created, createProductRole(t, productID))
	}

	// Full page in one shot.
	result, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Sort: []search.Sort[role.SortFieldProductRole]{
				{Field: role.SortFieldProductRoleCreatedAt, Direction: search.SortAscending},
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
	page1, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Sort: []search.Sort[role.SortFieldProductRole]{
				{Field: role.SortFieldProductRoleCreatedAt, Direction: search.SortAscending},
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
	page2, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Sort: []search.Sort[role.SortFieldProductRole]{
				{Field: role.SortFieldProductRoleCreatedAt, Direction: search.SortAscending},
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
	page3, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Sort: []search.Sort[role.SortFieldProductRole]{
				{Field: role.SortFieldProductRoleCreatedAt, Direction: search.SortAscending},
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
	pageOOB, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Pagination: search.Pagination{Limit: 10, Offset: 50},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestProductRoleRepositorySearchByProductIDSortDirections(t *testing.T) {
	productID := tenantProductChain(t)

	// Names chosen so ascending/descending name order is unambiguous.
	names := []string{"Alpha", "Bravo", "Charlie"}
	ids := make(map[string]string, len(names))
	for _, name := range names {
		entity := role.ProductRole{ProductID: productID, Name: name, Description: "fixture"}
		entity.GenerateID()
		created, err := repoCtx.ProductRoleRepository.Create(context.Background(), entity)
		require.NoError(t, err)
		ids[name] = created.ID
	}

	// created_at
	ascCreated, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Sort: []search.Sort[role.SortFieldProductRole]{
				{Field: role.SortFieldProductRoleCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, ascCreated.Items, 3)
	assert.Equal(t, []string{ids["Alpha"], ids["Bravo"], ids["Charlie"]},
		[]string{ascCreated.Items[0].ID, ascCreated.Items[1].ID, ascCreated.Items[2].ID})

	descCreated, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Sort: []search.Sort[role.SortFieldProductRole]{
				{Field: role.SortFieldProductRoleCreatedAt, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, descCreated.Items, 3)
	assert.Equal(t, []string{ids["Charlie"], ids["Bravo"], ids["Alpha"]},
		[]string{descCreated.Items[0].ID, descCreated.Items[1].ID, descCreated.Items[2].ID})

	// updated_at: same insertion order as created_at above (no updates happened),
	// so the ordering assertion mirrors created_at.
	ascUpdated, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Sort: []search.Sort[role.SortFieldProductRole]{
				{Field: role.SortFieldProductRoleUpdatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, ascUpdated.Items, 3)
	assert.Equal(t, []string{ids["Alpha"], ids["Bravo"], ids["Charlie"]},
		[]string{ascUpdated.Items[0].ID, ascUpdated.Items[1].ID, ascUpdated.Items[2].ID})

	descUpdated, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Sort: []search.Sort[role.SortFieldProductRole]{
				{Field: role.SortFieldProductRoleUpdatedAt, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, descUpdated.Items, 3)
	assert.Equal(t, []string{ids["Charlie"], ids["Bravo"], ids["Alpha"]},
		[]string{descUpdated.Items[0].ID, descUpdated.Items[1].ID, descUpdated.Items[2].ID})

	// name
	ascName, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Sort: []search.Sort[role.SortFieldProductRole]{
				{Field: role.SortFieldProductRoleName, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, ascName.Items, 3)
	assert.Equal(t, []string{ids["Alpha"], ids["Bravo"], ids["Charlie"]},
		[]string{ascName.Items[0].ID, ascName.Items[1].ID, ascName.Items[2].ID})

	descName, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Sort: []search.Sort[role.SortFieldProductRole]{
				{Field: role.SortFieldProductRoleName, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, descName.Items, 3)
	assert.Equal(t, []string{ids["Charlie"], ids["Bravo"], ids["Alpha"]},
		[]string{descName.Items[0].ID, descName.Items[1].ID, descName.Items[2].ID})
}

func TestProductRoleRepositorySearchByProductIDFilterByIDs(t *testing.T) {
	productID := tenantProductChain(t)

	first := createProductRole(t, productID)
	second := createProductRole(t, productID)
	_ = createProductRole(t, productID) // unrelated third role, must not match

	byID, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Filter:     &role.SearchProductRoleFilter{ProductRoleIDs: []string{first, second}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byID.Total)
	gotIDs := []string{byID.Items[0].ID, byID.Items[1].ID}
	assert.ElementsMatch(t, []string{first, second}, gotIDs)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Filter:     &role.SearchProductRoleFilter{ProductRoleIDs: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestProductRoleRepositorySearchByProductIDFilterByNames(t *testing.T) {
	productID := tenantProductChain(t)

	name := uniqueName(t, "Editor Role")
	entity := role.ProductRole{ProductID: productID, Name: name, Description: "fixture"}
	entity.GenerateID()
	matching, err := repoCtx.ProductRoleRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	_ = createProductRole(t, productID) // unrelated role, must not match

	// Names filter is a substring LIKE match (see the manual LIKE construction
	// in SearchByProductID), so a partial name is enough.
	result, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Filter:     &role.SearchProductRoleFilter{Names: []string{"Editor Role"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, matching.ID, result.Items[0].ID)

	// A name filter matching nothing returns an empty, not an error.
	none, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Filter:     &role.SearchProductRoleFilter{Names: []string{"NoSuchRoleName"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestProductRoleRepositorySearchByProductIDScopesToProduct(t *testing.T) {
	productID := tenantProductChain(t)
	otherProductID := tenantProductChain(t)

	inScope := createProductRole(t, productID)
	_ = createProductRole(t, otherProductID) // must never leak into the first product's results

	result, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, inScope, result.Items[0].ID)

	otherResult, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), otherProductID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
	for _, item := range otherResult.Items {
		assert.NotEqual(t, inScope, item.ID)
	}
}

// Pagination must page over roles, not over the role⋈permissions join: a page
// limit smaller than a role's permission count must still return every
// permission for the roles on that page (regression for permission-list
// truncation by LIMIT/OFFSET on the joined rows).
func TestProductRoleRepositorySearchByProductIDPermissionsNotTruncatedByPagination(t *testing.T) {
	productID := tenantProductChain(t)

	// product_role_resource_permissions.permission_name FK-references
	// product_resource_permissions(product_id, name), so the referenced
	// resource permissions must exist first.
	permNames := []string{"perm.one", "perm.two", "perm.three"}
	for _, name := range permNames {
		_, err := repoCtx.ProductResourcePermissionRepository.Create(
			context.Background(),
			resourcepermission.ProductResourcePermission{ProductID: productID, Name: name},
		)
		require.NoError(t, err)
	}

	entity := role.ProductRole{
		// A fresh product per test means a plain literal name is unique enough —
		// uniqueName's t.Name() prefix would push this past the 100-char column
		// limit on product_roles.name for this particularly long test name.
		ProductID:   productID,
		Name:        "Permission Heavy Role",
		Description: "fixture",
	}
	entity.GenerateID()
	entity.Permissions = make([]role.ProductRolePermission, len(permNames))
	for i, name := range permNames {
		perm := role.ProductRolePermission{ProductRoleID: entity.ID, ProductID: productID, PermissionName: name}
		perm.GenerateID()
		entity.Permissions[i] = perm
	}
	created, err := repoCtx.ProductRoleRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	result, err := repoCtx.ProductRoleRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
			Filter:     &role.SearchProductRoleFilter{ProductRoleIDs: []string{created.ID}},
			Pagination: search.Pagination{Limit: 1, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Len(t, result.Items[0].Permissions, 3)
}

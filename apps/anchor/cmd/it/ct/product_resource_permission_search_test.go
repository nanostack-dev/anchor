package ct_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestProductResourcePermissionSearchSuccess(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	permissions := []ct.CreateProductResourcePermissionRequest{
		{
			Name:          "file:read",
			Description:   new("Read file contents"),
			ScopeModifier: new("own"),
		},
		{
			Name:          "file:write",
			Description:   new("Write file contents"),
			ScopeModifier: new("team"),
		},
		{
			Name:        "file:delete",
			Description: new("Delete file contents"),
		},
		{
			Name:        "document:read",
			Description: new("Read document contents"),
		},
	}

	for _, permission := range permissions {
		createResp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
			ctx,
			testProduct.ProductID,
			permission,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, createResp.StatusCode())
		assert.NotNil(t, createResp.JSON201)
	}

	searchInput := ct.ProductResourcePermissionSearchRequest{
		Filter: &ct.ProductResourcePermissionFilter{},
	}

	resp, err := testOwnerClient(t).SearchProductResourcePermissionsWithResponse(
		ctx, testProduct.ProductID, searchInput,
	)
	require.NoError(t, err, "search product resource permissions request should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.GreaterOrEqual(
			t, len(resp.JSON200.Items), len(permissions), "should find all created permissions",
		)
		assert.NotNil(t, resp.JSON200.Total)
		assert.GreaterOrEqual(t, resp.JSON200.Total, int64(len(permissions)))
		for _, item := range resp.JSON200.Items {
			assert.Equal(t, testProduct.ProductID, item.ProductId)
			assert.NotEmpty(t, item.Name)
			assert.NotZero(t, item.CreatedAt)
			assert.NotZero(t, item.UpdatedAt)
		}
	}
}

func TestProductResourcePermissionSearchByNames(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	permissions := []ct.CreateProductResourcePermissionRequest{
		{
			Name:        "file:read",
			Description: new("Read file contents"),
		},
		{
			Name:        "file:write",
			Description: new("Write file contents"),
		},
		{
			Name:        "document:read",
			Description: new("Read document contents"),
		},
	}

	for _, permission := range permissions {
		createResp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
			ctx,
			testProduct.ProductID,
			permission,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, createResp.StatusCode())
	}

	searchNames := []string{"file:read", "document:read"}
	searchInput := ct.ProductResourcePermissionSearchRequest{
		Filter: &ct.ProductResourcePermissionFilter{
			Names: searchNames,
		},
	}

	resp, err := testOwnerClient(t).SearchProductResourcePermissionsWithResponse(
		ctx, testProduct.ProductID, searchInput,
	)
	require.NoError(t, err, "search product resource permissions by names should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.Len(
			t, resp.JSON200.Items, len(searchNames),
			"should find exact number of requested permissions",
		)
		assert.NotNil(t, resp.JSON200.Total)
		assert.Equal(t, int64(len(searchNames)), resp.JSON200.Total)

		foundNames := make(map[string]bool)
		for _, item := range resp.JSON200.Items {
			foundNames[item.Name] = true
			assert.Contains(
				t, searchNames, item.Name, "returned permission should be in search criteria",
			)
			assert.Equal(t, testProduct.ProductID, item.ProductId)
		}

		for _, name := range searchNames {
			assert.True(t, foundNames[name], "searched permission %s should be found", name)
		}
	}
}

func TestProductResourcePermissionSearchWithNonExistentNames(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	searchNames := []string{"non:existent", "another:missing"}
	searchInput := ct.ProductResourcePermissionSearchRequest{
		Filter: &ct.ProductResourcePermissionFilter{
			Names: searchNames,
		},
	}

	resp, err := testOwnerClient(t).SearchProductResourcePermissionsWithResponse(
		ctx, testProduct.ProductID, searchInput,
	)
	require.NoError(t, err, "search for non-existent permissions should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.Empty(t, resp.JSON200.Items, "should find no permissions")
		assert.NotNil(t, resp.JSON200.Total)
		assert.Equal(t, int64(0), resp.JSON200.Total)
	}
}

func TestProductResourcePermissionSearchWithPagination(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	permissionCount := 5
	for range permissionCount {
		permission := ct.CreateProductResourcePermissionRequest{
			Name:        itshared.Faker.Lorem().Word() + ":read",
			Description: new(itshared.Faker.Lorem().Sentence(5)),
		}

		createResp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
			ctx,
			testProduct.ProductID,
			permission,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, createResp.StatusCode())
	}

	limit := int32(2)
	offset := int32(1)
	searchInput := ct.ProductResourcePermissionSearchRequest{
		Filter: &ct.ProductResourcePermissionFilter{},
		Pagination: &ct.PaginationRequest{
			Limit:  &limit,
			Offset: &offset,
		},
	}

	resp, err := testOwnerClient(t).SearchProductResourcePermissionsWithResponse(
		ctx, testProduct.ProductID, searchInput,
	)
	require.NoError(t, err, "search with pagination should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.LessOrEqual(t, len(resp.JSON200.Items), int(limit), "should respect limit")
		assert.NotNil(t, resp.JSON200.Total)
		assert.GreaterOrEqual(t, resp.JSON200.Total, int64(permissionCount))
		assert.NotNil(t, resp.JSON200.Count)
		assert.Equal(t, int(limit), resp.JSON200.Count, "should return correct count for page")
	}
}

func TestProductResourcePermissionSearchWithSorting(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	permissions := []string{"apple:read", "banana:read", "cherry:read"}
	for _, name := range permissions {
		permission := ct.CreateProductResourcePermissionRequest{
			Name:        name,
			Description: new("Test permission"),
		}

		createResp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
			ctx,
			testProduct.ProductID,
			permission,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, createResp.StatusCode())
	}

	name := ct.ProductResourcePermissionSearchRequestSortByName
	searchInput := ct.ProductResourcePermissionSearchRequest{
		Filter: &ct.ProductResourcePermissionFilter{},
		SortBy: &name,
	}

	resp, err := testOwnerClient(t).SearchProductResourcePermissionsWithResponse(
		ctx, testProduct.ProductID, searchInput,
	)
	require.NoError(t, err, "search with sorting should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.GreaterOrEqual(t, len(resp.JSON200.Items), len(permissions))

		var foundPermissions []string
		for _, item := range resp.JSON200.Items {
			if slices.Contains(permissions, item.Name) {
				foundPermissions = append(foundPermissions, item.Name)
			}
		}

		assert.Len(t, foundPermissions, len(permissions), "should find all test permissions")

		for i := 1; i < len(foundPermissions); i++ {
			assert.LessOrEqual(
				t, foundPermissions[i-1], foundPermissions[i],
				"permissions should be sorted alphabetically",
			)
		}
	}
}

func TestProductResourcePermissionSearchWithNonExistentProduct(t *testing.T) {
	ctx := context.Background()

	nonExistentProductID := ids.MustNew("prod")

	searchInput := ct.ProductResourcePermissionSearchRequest{
		Filter: &ct.ProductResourcePermissionFilter{},
	}

	resp, err := testOwnerClient(t).SearchProductResourcePermissionsWithResponse(
		ctx, nonExistentProductID, searchInput,
	)
	require.NoError(t, err, "search for non-existent product should not error")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
}

func TestProductResourcePermissionSearchEmptyFilter(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	permission := ct.CreateProductResourcePermissionRequest{
		Name:        "test:read",
		Description: new("Test permission"),
	}

	createResp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx,
		testProduct.ProductID,
		permission,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode())

	searchInput := ct.ProductResourcePermissionSearchRequest{}

	resp, err := testOwnerClient(t).SearchProductResourcePermissionsWithResponse(
		ctx, testProduct.ProductID, searchInput,
	)
	require.NoError(t, err, "search with empty request should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.GreaterOrEqual(
			t, len(resp.JSON200.Items), 1, "should find at least the created permission",
		)
		assert.NotNil(t, resp.JSON200.Total)
		assert.GreaterOrEqual(t, resp.JSON200.Total, int64(1))
	}
}

func TestProductResourcePermissionSearchMixedNamesFilter(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	existingPermissions := []string{"file:read", "file:write", "document:read"}
	for _, name := range existingPermissions {
		permission := ct.CreateProductResourcePermissionRequest{
			Name:        name,
			Description: new("Test permission"),
		}

		createResp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
			ctx,
			testProduct.ProductID,
			permission,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, createResp.StatusCode())
	}

	searchNames := []string{"file:read", "non:existent", "document:read", "another:missing"}
	searchInput := ct.ProductResourcePermissionSearchRequest{
		Filter: &ct.ProductResourcePermissionFilter{
			Names: searchNames,
		},
	}

	resp, err := testOwnerClient(t).SearchProductResourcePermissionsWithResponse(
		ctx, testProduct.ProductID, searchInput,
	)
	require.NoError(t, err, "search with mixed names should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.Len(t, resp.JSON200.Items, 2, "should find only existing permissions")
		assert.NotNil(t, resp.JSON200.Total)
		assert.Equal(t, int64(2), resp.JSON200.Total)

		foundNames := make(map[string]bool)
		for _, item := range resp.JSON200.Items {
			foundNames[item.Name] = true
			assert.Contains(t, []string{"file:read", "document:read"}, item.Name)
		}

		assert.True(t, foundNames["file:read"])
		assert.True(t, foundNames["document:read"])
	}
}

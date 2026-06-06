package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductRole_Search(t *testing.T) {
	ctx := context.Background()
	testCtx := createTestProductContext(t)
	productID := testCtx.ProductID

	role1Name := "Editor_" + ids.MustNew("test")
	role2Name := "Viewer_" + ids.MustNew("test")
	role3Name := "Admin_" + ids.MustNew("test")

	createResp1, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
		ctx, productID, ct.CreateProductRoleJSONRequestBody{
			Name: role1Name,
		},
	)
	require.NoError(t, err)
	require.Equal(
		t, http.StatusCreated,
		createResp1.StatusCode(),
	)

	createResp2, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
		ctx, productID, ct.CreateProductRoleJSONRequestBody{
			Name: role2Name,
		},
	)
	require.NoError(t, err)
	require.Equal(
		t, http.StatusCreated,
		createResp2.StatusCode(),
	)

	createResp3, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
		ctx, productID, ct.CreateProductRoleJSONRequestBody{
			Name: role3Name,
		},
	)
	require.NoError(t, err)
	require.Equal(
		t, http.StatusCreated,
		createResp3.StatusCode(),
	)

	t.Run(
		"SearchAllProductRoles", func(t *testing.T) {
			searchResp, searchErr := testCtx.OwnerAuthenticatedClient().SearchProductRolesWithResponse(
				ctx, productID, ct.SearchProductRolesJSONRequestBody{},
			)
			require.NoError(t, searchErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.GreaterOrEqual(t, len(searchResp.JSON200.Items), 3)
			assert.GreaterOrEqual(t, searchResp.JSON200.Total, int64(3))
		},
	)

	t.Run(
		"SearchProductRolesByNames", func(t *testing.T) {
			searchResp, nameFilterErr := testCtx.OwnerAuthenticatedClient().SearchProductRolesWithResponse(
				ctx, productID, ct.SearchProductRolesJSONRequestBody{
					Filter: &ct.ProductRoleFilter{
						Names: []string{role1Name, role3Name},
					},
				},
			)
			require.NoError(t, nameFilterErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.Len(t, searchResp.JSON200.Items, 2)

			roleNames := make(map[string]bool)
			for _, role := range searchResp.JSON200.Items {
				roleNames[role.Name] = true
			}
			assert.True(t, roleNames[role1Name])
			assert.True(t, roleNames[role3Name])
			assert.False(t, roleNames[role2Name])
		},
	)

	t.Run(
		"SearchProductRolesByIds", func(t *testing.T) {
			roleIDs := []string{createResp1.JSON201.Id, createResp2.JSON201.Id}
			searchResp, idFilterErr := testCtx.OwnerAuthenticatedClient().SearchProductRolesWithResponse(
				ctx, productID, ct.SearchProductRolesJSONRequestBody{
					Filter: &ct.ProductRoleFilter{
						Ids: roleIDs,
					},
				},
			)
			require.NoError(t, idFilterErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.Len(t, searchResp.JSON200.Items, 2)

			foundIDs := make(map[string]bool)
			for _, role := range searchResp.JSON200.Items {
				foundIDs[role.Id] = true
			}
			assert.True(t, foundIDs[createResp1.JSON201.Id])
			assert.True(t, foundIDs[createResp2.JSON201.Id])
		},
	)

	t.Run(
		"SearchProductRolesWithPagination", func(t *testing.T) {
			limit := int32(2)
			offset := int32(0)
			pagination := ct.PaginationRequest{
				Limit:  &limit,
				Offset: &offset,
			}
			searchResp, paginationErr := testCtx.OwnerAuthenticatedClient().SearchProductRolesWithResponse(
				ctx, productID, ct.SearchProductRolesJSONRequestBody{
					Pagination: &pagination,
				},
			)
			require.NoError(t, paginationErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.LessOrEqual(t, len(searchResp.JSON200.Items), 2)
		},
	)

	t.Run(
		"SearchProductRolesWithSorting", func(t *testing.T) {
			sortBy := ct.ProductRoleSearchRequestSortByName
			sortDirection := ct.ASC
			searchResp, sortErr := testCtx.OwnerAuthenticatedClient().SearchProductRolesWithResponse(
				ctx, productID, ct.SearchProductRolesJSONRequestBody{
					SortBy:        &sortBy,
					SortDirection: &sortDirection,
				},
			)
			require.NoError(t, sortErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)

			if len(searchResp.JSON200.Items) > 1 {
				for i := 1; i < len(searchResp.JSON200.Items); i++ {
					assert.LessOrEqual(
						t, searchResp.JSON200.Items[i-1].Name, searchResp.JSON200.Items[i].Name,
					)
				}
			}
		},
	)

	t.Run(
		"SearchProductRolesWithFullTextSearch", func(t *testing.T) {
			fullTextSearch := "Edit"
			searchResp, fullTextErr := testCtx.OwnerAuthenticatedClient().SearchProductRolesWithResponse(
				ctx, productID, ct.SearchProductRolesJSONRequestBody{
					FullTextSearch: &fullTextSearch,
				},
			)
			require.NoError(t, fullTextErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)

			found := false
			for _, role := range searchResp.JSON200.Items {
				if role.Name == role1Name {
					found = true
					break
				}
			}
			assert.True(t, found)
		},
	)

	t.Run(
		"SearchProductRolesNoResults", func(t *testing.T) {
			searchResp, noResultsErr := testCtx.OwnerAuthenticatedClient().SearchProductRolesWithResponse(
				ctx, productID, ct.SearchProductRolesJSONRequestBody{
					Filter: &ct.ProductRoleFilter{
						Names: []string{"NonexistentRole"},
					},
				},
			)
			require.NoError(t, noResultsErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.Empty(t, searchResp.JSON200.Items)
			assert.Equal(t, int64(0), searchResp.JSON200.Total)
		},
	)

	//TODO: Good use case check for this later
	// t.Run(
	//	"SearchProductRolesForNonexistentProduct", func(t *testing.T) {
	//		// nonExistentProductID := ids.MustNew("prd")
	//		// searchResp, err := testCtx.OwnerAuthenticatedClient().SearchProductRolesWithResponse(
	//		//	ctx, nonExistentProductID, client.SearchProductRolesJSONRequestBody{},
	//		//)
	//		////require.NoError(t, err)
	//		////assert.NotNil(t, searchResp.JSON401)
	//	},
	//)
}

// Pagination must page over roles, not over the role⋈permissions join. A page
// limit smaller than a role's permission count must still return ALL of that
// role's permissions (regression for permission-list truncation).
func TestProductRole_Search_PermissionsNotTruncatedByPagination(t *testing.T) {
	ctx := context.Background()
	testCtx := createTestProductContext(t)
	productID := testCtx.ProductID

	perms := testCtx.CreateDefaultProductResourcePermissions(t)
	require.Greater(t, len(perms), 1, "need more than one permission to exercise truncation")

	permNames := make([]string, len(perms))
	for i, p := range perms {
		permNames[i] = p.Name
	}

	roleName := "PermHeavy_" + ids.MustNew("test")
	createResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
		ctx, productID, ct.CreateProductRoleJSONRequestBody{
			Name:        roleName,
			Permissions: permNames,
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)
	roleID := createResp.JSON201.Id

	limit := int32(1)
	offset := int32(0)
	searchResp, searchErr := testCtx.OwnerAuthenticatedClient().SearchProductRolesWithResponse(
		ctx, productID, ct.SearchProductRolesJSONRequestBody{
			Filter:     &ct.ProductRoleFilter{Ids: []string{roleID}},
			Pagination: &ct.PaginationRequest{Limit: &limit, Offset: &offset},
		},
	)
	require.NoError(t, searchErr)
	require.Equal(t, http.StatusOK, searchResp.StatusCode())
	require.NotNil(t, searchResp.JSON200)
	require.Len(t, searchResp.JSON200.Items, 1)
	assert.Len(
		t, searchResp.JSON200.Items[0].Permissions, len(permNames),
		"all permissions must be returned even when the page limit is smaller than the permission count",
	)
}

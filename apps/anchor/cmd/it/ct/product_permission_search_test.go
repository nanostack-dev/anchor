package ct_test

import (
	"context"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductPermissions_Search(t *testing.T) {
	ctx := context.Background()
	testCtx := createTestProductContext(t)

	productID := testCtx.ProductID

	orgRead := "organization:read"
	orgMemberRead := "organization_member:read"
	orgMemberWrite := "organization_member:create"
	orgMemberUpdate := "organization_member:update"
	orgMemberDelete := "organization_member:delete"

	t.Run(
		"Search existing permissions", func(t *testing.T) {
			searchResp, err := testCtx.OwnerAuthenticatedClient().SearchProductPermissionsWithResponse(
				ctx, productID, ct.SearchProductPermissionsJSONRequestBody{
					Filter: &ct.ProductPermissionFilter{
						Names: []string{orgRead},
					},
				},
			)
			require.NoError(t, err, "search product permissions request should not error")
			assert.Equal(
				t, 200, searchResp.StatusCode(), "search product permissions should return 200 OK",
			)
			if assert.NotEmpty(t, searchResp.JSON200, "search results should not be empty") {
				assert.Equal(t, orgRead, searchResp.JSON200.Items[0].Name)
			}
		},
	)

	t.Run(
		"Search existing permissions with different case", func(t *testing.T) {
			searchResp, err := testCtx.OwnerAuthenticatedClient().SearchProductPermissionsWithResponse(
				ctx, productID, ct.SearchProductPermissionsJSONRequestBody{
					Filter: &ct.ProductPermissionFilter{
						Names: []string{"ORGANIZATION:READ"},
					},
				},
			)
			require.NoError(t, err, "search product permissions request should not error")
			assert.Equal(t, 200, searchResp.StatusCode())
			if assert.NotEmpty(t, searchResp.JSON200, "search results should not be empty") {
				assert.Equal(t, orgRead, searchResp.JSON200.Items[0].Name)
			}
		},
	)

	t.Run(
		"Search by multiple permissions", func(t *testing.T) {
			searchResp, err := testCtx.OwnerAuthenticatedClient().SearchProductPermissionsWithResponse(
				ctx, productID, ct.SearchProductPermissionsJSONRequestBody{
					FullTextSearch: ptr.Ptr("organization_m"),
				},
			)
			require.NoError(t, err, "search product permissions request should not error")
			assert.Equal(
				t, 200, searchResp.StatusCode(), "search product permissions should return 200 OK",
			)
			assert.Len(
				t, searchResp.JSON200.Items, 4,
				"search results should contain organization permissions",
			)
			assert.ElementsMatch(
				t,
				[]string{
					orgMemberRead,
					orgMemberWrite,
					orgMemberUpdate,
					orgMemberDelete,
				},
				slicex.Map(
					searchResp.JSON200.Items, func(item ct.ProductPermissionResponse) string {
						return item.Name
					},
				),
			)
		},
	)

	t.Run(
		"Search non-existent permission", func(t *testing.T) {
			searchResp, err := testCtx.OwnerAuthenticatedClient().SearchProductPermissionsWithResponse(
				ctx, productID, ct.SearchProductPermissionsJSONRequestBody{
					Filter: &ct.ProductPermissionFilter{
						Names: []string{"nonexistent:permission"},
					},
				},
			)
			require.NoError(t, err, "search product permissions request should not error")
			assert.Equal(
				t, 200, searchResp.StatusCode(), "search product permissions should return 200 OK",
			)
			assert.NotNil(t, searchResp.JSON200, "search response should not be nil")
			if searchResp.JSON200 != nil {
				assert.Empty(
					t, searchResp.JSON200.Items,
					"search results should be empty for non-existent permission",
				)
				assert.Empty(
					t, searchResp.JSON200.Items,
					"search results should contain no items for non-existent permission",
				)
			}
		},
	)
}

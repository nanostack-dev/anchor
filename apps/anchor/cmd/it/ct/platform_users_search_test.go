package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchPlatformUsers(t *testing.T) {
	ctx := context.Background()

	t.Run(
		"SuccessfulSearch", func(t *testing.T) {
			searchReq := ct.SearchPlatformUsersJSONRequestBody{}

			resp, err := testOwnerClient(t).SearchPlatformUsersWithResponse(
				ctx, searchReq,
			)
			require.NoError(t, err, "search platform users request should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.NotNil(t, resp.JSON200)
			assert.NotNil(t, resp.JSON200.Items)
			assert.GreaterOrEqual(
				t, len(resp.JSON200.Items), 1,
				"should return at least the owner productuserservice",
			)
			assert.GreaterOrEqual(t, resp.JSON200.Total, int64(1))
			assert.GreaterOrEqual(t, resp.JSON200.Count, 1)
		},
	)

	t.Run(
		"SearchWithPagination", func(t *testing.T) {
			limit := int32(1)
			offset := int32(0)
			pagination := ct.PaginationRequest{
				Limit:  &limit,
				Offset: &offset,
			}
			searchReq := ct.SearchPlatformUsersJSONRequestBody{
				Pagination: &pagination,
			}

			resp, err := testOwnerClient(t).SearchPlatformUsersWithResponse(
				ctx, searchReq,
			)
			require.NoError(t, err, "search platform users with pagination should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.NotNil(t, resp.JSON200)
			assert.LessOrEqual(t, len(resp.JSON200.Items), 1, "should respect limit parameter")
		},
	)

	t.Run(
		"SearchWithEmailFilter", func(t *testing.T) {
			email := testOwnerUser(t).Email
			filter := ct.PlatformUserFilter{
				Emails: []string{
					email,
				},
			}
			searchReq := ct.SearchPlatformUsersJSONRequestBody{
				Filter: &filter,
			}

			resp, err := testOwnerClient(t).SearchPlatformUsersWithResponse(
				ctx, searchReq,
			)
			require.NoError(t, err, "search platform users with email filter should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.NotNil(t, resp.JSON200)
			if len(resp.JSON200.Items) > 0 {
				assert.Equal(t, resp.JSON200.Items[0].Email, testOwnerUser(t).Email)
			}
		},
	)

	t.Run(
		"SearchWithRoleFilter", func(t *testing.T) {
			roles := []ct.PlatformUserRole{ct.OWNER}
			filter := ct.PlatformUserFilter{
				Roles: roles,
			}
			searchReq := ct.SearchPlatformUsersJSONRequestBody{
				Filter: &filter,
			}

			resp, err := testOwnerClient(t).SearchPlatformUsersWithResponse(
				ctx, searchReq,
			)
			require.NoError(t, err, "search platform users with role filter should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.NotNil(t, resp.JSON200)
			if len(resp.JSON200.Items) > 0 {
				assert.Equal(t, ct.OWNER, resp.JSON200.Items[0].Role)
			}
		},
	)

	t.Run(
		"SearchWithSorting", func(t *testing.T) {
			sortBy := ct.PlatformUserSearchRequestSortByEmail
			sortDirection := ct.ASC
			searchReq := ct.SearchPlatformUsersJSONRequestBody{
				SortBy:        &sortBy,
				SortDirection: &sortDirection,
			}

			resp, err := testOwnerClient(t).SearchPlatformUsersWithResponse(
				ctx, searchReq,
			)
			require.NoError(t, err, "search platform users with sorting should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.NotNil(t, resp.JSON200)
		},
	)

	t.Run(
		"SearchWithFullTextSearch", func(t *testing.T) {
			fullTextSearch := "platform"
			searchReq := ct.SearchPlatformUsersJSONRequestBody{
				FullTextSearch: &fullTextSearch,
			}

			resp, err := testOwnerClient(t).SearchPlatformUsersWithResponse(
				ctx, searchReq,
			)
			require.NoError(t, err, "search platform users with full text search should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.NotNil(t, resp.JSON200)
		},
	)
}

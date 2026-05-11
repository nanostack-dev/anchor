package ct_test

import (
	"context"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"net/http"
	"testing"

	"github.com/nanostack-dev/shared/toolkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCurrentUserPlatformUser(t *testing.T) {
	ctx := context.Background()

	t.Run(
		"SuccessfulGetCurrentUserAsOwner", func(t *testing.T) {
			resp, err := testOwnerClient(t).GetCurrentUserWithResponse(ctx)
			require.NoError(t, err, "get current productuserservice request should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.NotNil(t, resp.JSON200)
			assert.Equal(t, resp.JSON200.Email, testOwnerUser(t).Email)
			assert.NotEmpty(t, resp.JSON200.Id)
		},
	)

	t.Run(
		"SuccessfulGetCurrentUserAsAdmin", func(t *testing.T) {
			testUser := createPlatformAdmin(t)
			assert.NotEmpty(
				t, testUser.AccessToken, "test productuserservice should have access token",
			)

			resp, err := testUser.AuthenticatedClient.GetCurrentUserWithResponse(ctx)
			require.NoError(t, err, "get current productuserservice request should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.NotNil(t, resp.JSON200)
			assert.Equal(t, resp.JSON200.Email, testUser.Email)
			assert.NotEmpty(t, resp.JSON200.Id)
		},
	)

	t.Run(
		"GetCurrentUserAfterUserDeletion", func(t *testing.T) {
			testUser := createPlatformAdmin(t)
			assert.NotEmpty(
				t, testUser.AccessToken, "test productuserservice should have access token",
			)
			resp, err := testUser.AuthenticatedClient.GetCurrentUserWithResponse(ctx)
			require.NoError(t, err, "get current productuserservice request should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.NotNil(t, resp.JSON200)

			response, err := testUser.AuthenticatedClient.SearchPlatformUsersWithResponse(
				ctx, ct.SearchPlatformUsersJSONRequestBody{
					Filter: &ct.PlatformUserFilter{
						Emails: []string{
							resp.JSON200.Email,
						},
					},
					Pagination: &ct.PaginationRequest{
						Limit:  toolkit.Ptr(int32(1)),
						Offset: toolkit.Ptr(int32(0)),
					},
				},
			)
			require.NoError(t, err, "search for created productuserservice should not error")
			assert.Equal(t, http.StatusOK, response.StatusCode())
			assert.NotNil(t, response.JSON200)
			assert.Len(t, response.JSON200.Items, 1, "should find exactly one productuserservice")
			testTenant(t).OwnerClient.DeletePlatformUser(
				ctx, response.JSON200.Items[0].Id,
			)

			resp2, err := testUser.AuthenticatedClient.GetCurrentUserWithResponse(ctx)
			require.NoError(t, err, "get current productuserservice request should not error")
			// The response depends on how token invalidation is handled
			assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode())
		},
	)
}

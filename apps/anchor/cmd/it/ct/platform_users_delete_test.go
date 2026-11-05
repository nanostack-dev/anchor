package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

func TestDeletePlatformUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run(
		"Deleting nonexistent user succeeds", func(t *testing.T) {
			nonExistentUserID := ids.MustNew("puser")

			resp, err := testOwnerClient(t).DeletePlatformUserWithResponse(
				ctx, nonExistentUserID,
			)
			require.NoError(
				t, err, "delete non-existent productuserservice request should not error",
			)
			assert.Equal(t, http.StatusNoContent, resp.StatusCode())
		},
	)

	t.Run(
		"Deleting a platform user removes the newly created user", func(t *testing.T) {
			// Create a test admin productuserservice first
			testUser := createPlatformAdmin(t)
			assert.NotEmpty(
				t, testUser.AccessToken, "test productuserservice should have access token",
			)

			// Search for the created productuserservice to get their platform productuserservice ID
			email := testUser.Email
			filter := ct.PlatformUserFilter{
				Emails: []string{
					email,
				},
			}
			searchReq := ct.SearchPlatformUsersJSONRequestBody{
				Filter: &filter,
			}

			searchResp, err := testOwnerClient(t).SearchPlatformUsersWithResponse(
				ctx, searchReq,
			)
			require.NoError(t, err, "search for created productuserservice should not error")
			assert.Equal(t, http.StatusOK, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			if !assert.GreaterOrEqual(
				t, len(searchResp.JSON200.Items), 1, "should find the created productuserservice",
			) {
				return
			}

			createdUserID := searchResp.JSON200.Items[0].Id

			// Now delete the productuserservice
			resp, err := testOwnerClient(t).DeletePlatformUserWithResponse(
				ctx, createdUserID,
			)
			require.NoError(t, err, "delete created productuserservice request should not error")
			assert.Equal(t, http.StatusNoContent, resp.StatusCode())

			// Verify the productuserservice is deleted by trying to get it
			getResp, err := testOwnerClient(t).GetPlatformUserWithResponse(
				ctx, createdUserID,
			)
			require.NoError(t, err, "get deleted productuserservice request should not error")
			assert.Equal(t, http.StatusNotFound, getResp.StatusCode())
		},
	)

	t.Run(
		"Self should deletion fail", func(t *testing.T) {
			// Search for the owner productuserservice to get their platform productuserservice ID
			email := testOwnerUser(t).Email
			filter := ct.PlatformUserFilter{
				Emails: []string{
					email,
				},
			}
			searchReq := ct.SearchPlatformUsersJSONRequestBody{
				Filter: &filter,
			}

			searchResp, err := testOwnerClient(t).SearchPlatformUsersWithResponse(
				ctx, searchReq,
			)
			require.NoError(t, err, "search for owner productuserservice should not error")
			assert.Equal(t, http.StatusOK, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			if !assert.GreaterOrEqual(
				t, len(searchResp.JSON200.Items), 1, "should find the owner productuserservice",
			) {
				return
			}

			ownerUserID := searchResp.JSON200.Items[0].Id

			// Try to delete self (should fail)
			resp, err := testOwnerClient(t).DeletePlatformUserWithResponse(
				ctx, ownerUserID,
			)
			require.NoError(t, err, "delete self request should not error")
			assert.Equal(
				t,
				http.StatusForbidden, resp.StatusCode(),
			)
		},
	)
}

func TestDeletePlatformUserKeepsTheOwner(t *testing.T) {
	t.Parallel()

	t.Run("An admin cannot delete the tenant owner, who keeps working access", func(t *testing.T) {
		t.Parallel()
		state := itdsl.Given(t).
			Tenant(itdsl.TenantOpts{Alias: "tenant.ownerdel", Isolated: true}).
			PlatformAdmin(itdsl.PlatformAdminOpts{Alias: "admin.ownerdel", TenantAlias: "tenant.ownerdel"}).
			Build()
		owner := state.Tenant("tenant.ownerdel").OwnerUser
		admin := state.PlatformUser("admin.ownerdel")

		deleted, err := admin.AuthenticatedClient.DeletePlatformUserWithResponse(t.Context(), owner.ID)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, deleted.StatusCode())

		self, err := owner.AuthenticatedClient.GetCurrentUserWithResponse(t.Context())
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, self.StatusCode())
	})
}

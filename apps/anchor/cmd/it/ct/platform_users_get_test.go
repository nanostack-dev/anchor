package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	itdsl "anchor/cmd/it/shared/dsl"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nanostack-dev/shared/toolkit"
)

const (
	tenantMainAlias    = "tenant.main"
	platformAdminAlias = "platform.admin"
)

func TestGetPlatformUser(t *testing.T) {
	ctx := context.Background()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: tenantMainAlias}).
		PlatformAdmin(itdsl.PlatformAdminOpts{Alias: platformAdminAlias, TenantAlias: tenantMainAlias}).
		Build()

	ownerClient := state.Tenant(tenantMainAlias).OwnerClient

	t.Run(
		"GetNonExistentUser", func(t *testing.T) {
			nonExistentUserID := toolkit.NewID("puser")

			resp, err := ownerClient.GetPlatformUserWithResponse(
				ctx, nonExistentUserID,
			)
			require.NoError(t, err, "get non-existent productuserservice request should not error")
			assert.Equal(t, http.StatusNotFound, resp.StatusCode())
		},
	)

	t.Run(
		"GetWithInvalidUserID", func(t *testing.T) {
			invalidUserID := "invalid-productuserservice-id"

			resp, err := ownerClient.GetPlatformUserWithResponse(
				ctx, invalidUserID,
			)
			require.NoError(
				t, err, "get with invalid productuserservice ID request should not error",
			)
			assert.GreaterOrEqual(t, resp.StatusCode(), 400, "should return client error")
		},
	)

	t.Run(
		"SuccessfulGetOfExistingUser", func(t *testing.T) {
			testUser := state.PlatformUser(platformAdminAlias)
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

			searchResp, err := ownerClient.SearchPlatformUsersWithResponse(
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

			// Now get the productuserservice
			resp, err := ownerClient.GetPlatformUserWithResponse(
				ctx, createdUserID,
			)
			require.NoError(t, err, "get created productuserservice request should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			if assert.NotNil(t, resp.JSON200) {
				assert.Equal(t, createdUserID, resp.JSON200.Id)
				assert.Equal(t, resp.JSON200.Email, testUser.Email)
				assert.Equal(t, ct.ADMIN, resp.JSON200.Role)
				assert.NotEmpty(t, resp.JSON200.UserId)
				assert.NotEmpty(t, resp.JSON200.TenantId)
				assert.NotZero(t, resp.JSON200.CreatedAt)
				assert.NotZero(t, resp.JSON200.UpdatedAt)
			}
		},
	)

	t.Run(
		"GetOwnerUser", func(t *testing.T) {
			email := state.Tenant(tenantMainAlias).OwnerUser.Email
			filter := ct.PlatformUserFilter{
				Roles: []ct.PlatformUserRole{ct.OWNER},
				Emails: []string{
					email,
				},
			}
			searchReq := ct.SearchPlatformUsersJSONRequestBody{
				Filter: &filter,
			}

			searchResp, err := ownerClient.SearchPlatformUsersWithResponse(
				ctx, searchReq,
			)
			require.NoError(t, err, "search for owner productuserservice should not error")
			assert.Equal(t, http.StatusOK, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			if !assert.Len(
				t, searchResp.JSON200.Items, 1, "should find the owner productuserservice",
			) {
				return
			}

			ownerUserID := searchResp.JSON200.Items[0].Id
			resp, err := ownerClient.GetPlatformUserWithResponse(
				ctx, ownerUserID,
			)
			require.NoError(t, err, "get owner productuserservice request should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			if assert.NotNil(t, resp.JSON200) {
				assert.Equal(t, ownerUserID, resp.JSON200.Id)
				assert.Equal(t, resp.JSON200.Email, email)
				assert.Equal(t, ct.OWNER, resp.JSON200.Role)
				assert.NotEmpty(t, resp.JSON200.UserId)
				assert.NotEmpty(t, resp.JSON200.TenantId)
				assert.NotZero(t, resp.JSON200.CreatedAt)
				assert.NotZero(t, resp.JSON200.UpdatedAt)
			}
		},
	)
}

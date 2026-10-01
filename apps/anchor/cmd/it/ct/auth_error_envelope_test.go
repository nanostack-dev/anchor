package ct_test

import (
	"net/http"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

func TestAuthMissingCredentialReturnsJSONErrorEnvelope(t *testing.T) {
	t.Parallel()

	resp, err := testTenant(t).NoAuthClient.GetCurrentUserWithResponse(t.Context())
	require.NoError(t, err)

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
	assert.Contains(t, resp.HTTPResponse.Header.Get("Content-Type"), "application/json")
	require.NotNil(t, resp.JSON401, string(resp.Body))
	require.NotEmpty(t, resp.JSON401.Errors)
	assert.Equal(t, "UNAUTHORIZED", resp.JSON401.Errors[0].Code)
}

func TestAuthUnknownProductReturnsJSONErrorEnvelope(t *testing.T) {
	t.Parallel()

	resp, err := testOwnerClient(t).GetProductWithResponse(t.Context(), ids.MustNew("prd"))
	require.NoError(t, err)

	require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	assert.Contains(t, resp.HTTPResponse.Header.Get("Content-Type"), "application/json")
	require.NotNil(t, resp.JSON404, string(resp.Body))
	require.NotEmpty(t, resp.JSON404.Errors)
	assert.Equal(t, "PRODUCT_NOT_FOUND", resp.JSON404.Errors[0].Code)
}

func TestDeletedPlatformUserTokenReturnsJSONErrorEnvelope(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "tenant.deleted-user"}).
		PlatformAdmin(itdsl.PlatformAdminOpts{Alias: "admin.deleted", TenantAlias: "tenant.deleted-user"}).
		Build()
	deletedUser := state.PlatformUser("admin.deleted")
	ownerClient := state.Tenant("tenant.deleted-user").OwnerClient

	deleteResp, err := ownerClient.DeletePlatformUserWithResponse(ctx, deletedUser.ID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, deleteResp.StatusCode(), string(deleteResp.Body))

	resp, err := deletedUser.AuthenticatedClient.GetCurrentUserWithResponse(ctx)
	require.NoError(t, err)

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
	assert.Contains(t, resp.HTTPResponse.Header.Get("Content-Type"), "application/json")
	require.NotNil(t, resp.JSON401, string(resp.Body))
	require.NotEmpty(t, resp.JSON401.Errors, string(resp.Body))
	assert.Equal(t, "UNAUTHORIZED", resp.JSON401.Errors[0].Code)
}

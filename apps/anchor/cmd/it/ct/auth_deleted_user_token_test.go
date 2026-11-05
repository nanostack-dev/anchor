package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
)

func TestDeletedPlatformUserTokenStopsWorking(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	tenantAlias := "tenant.deleted-user.revoke." + itshared.Faker.UUID().V4()
	const removedAlias = "admin.deleted-user.removed"
	const productAlias = "product.deleted-user.revoke"
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: tenantAlias}).
		PlatformAdmin(itdsl.PlatformAdminOpts{Alias: removedAlias, TenantAlias: tenantAlias}).
		Product(itdsl.ProductOpts{Alias: productAlias, TenantAlias: tenantAlias}).
		Build()
	owner := state.Tenant(tenantAlias).OwnerClient
	removed := state.PlatformUser(removedAlias)
	productID := state.Product(productAlias).ProductID

	before, err := removed.AuthenticatedClient.CreateProductAPIKeyWithResponse(
		ctx, productID, ct.CreateProductAPIKeyJSONRequestBody{
			Name:        "deleted-user-before-delete-" + itshared.Faker.UUID().V4(),
			Permissions: []string{"organization:read"},
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, before.StatusCode(), "admin bearer must work before deletion")

	deleteResp, err := owner.DeletePlatformUserWithResponse(ctx, removed.ID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, deleteResp.StatusCode(), string(deleteResp.Body))

	refresh := itdsl.Auth(t).RefreshRaw(removed.RefreshToken)
	require.Equal(t, http.StatusUnauthorized, refresh.StatusCode(), "deletion must end the session")

	after, err := removed.AuthenticatedClient.CreateProductAPIKeyWithResponse(
		ctx, productID, ct.CreateProductAPIKeyJSONRequestBody{
			Name:        "deleted-user-after-delete-" + itshared.Faker.UUID().V4(),
			Permissions: []string{"organization:read"},
		},
	)
	require.NoError(t, err)
	if after.JSON201 != nil {
		t.Errorf(
			"deleted platform user minted product API key id=%s obfuscated=%s permissions=%d",
			after.JSON201.Id, after.JSON201.ObfuscatedValue, len(after.JSON201.Permissions),
		)
	}
	assert.Equal(t, http.StatusUnauthorized, after.StatusCode(),
		"a deleted platform user's access token must be refused")
}

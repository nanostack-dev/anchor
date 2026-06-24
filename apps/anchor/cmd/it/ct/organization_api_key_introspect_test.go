package ct_test

import (
	"context"
	"net/http"
	"testing"

	itshared "anchor/cmd/it/shared"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationAPIKeyIntrospect(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	adminClient, _ := product.CreateAPIKeyClientWithScopes([]string{
		"organization_api_key:create",
		"organization_api_key:read",
	})

	permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
	description := itshared.Faker.Lorem().Sentence(4)
	org := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description)

	createResp, err := adminClient.CreateOrganizationAPIKeyWithResponse(
		ctx,
		product.ProductID,
		org.Id,
		ct.CreateOrganizationAPIKeyJSONRequestBody{
			Name:        "OrgKey-" + uuid.NewString(),
			Permissions: []string{permissions.FileRead},
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)
	clearKey := createResp.JSON201.Value

	t.Run("resolves the key's organization and permissions without an organization id", func(t *testing.T) {
		resp, introErr := adminClient.IntrospectOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			ct.IntrospectOrganizationAPIKeyJSONRequestBody{ApiKey: clearKey},
		)
		require.NoError(t, introErr)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)

		assert.Equal(t, createResp.JSON201.Id, resp.JSON200.ApiKey.Id)
		// Organization is derived from the key — never supplied by the caller.
		assert.Equal(t, org.Id, resp.JSON200.ApiKey.OrganizationId)
		assert.ElementsMatch(t, []string{permissions.FileRead}, resp.JSON200.Permissions)
		assert.Empty(t, resp.JSON200.MissingPrivileges)
	})

	t.Run("returns 200 when optional required scopes are satisfied", func(t *testing.T) {
		scopes := []string{permissions.FileRead}
		resp, introErr := adminClient.IntrospectOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			ct.IntrospectOrganizationAPIKeyJSONRequestBody{ApiKey: clearKey, RequiredScopes: &scopes},
		)
		require.NoError(t, introErr)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Empty(t, resp.JSON200.MissingPrivileges)
	})

	t.Run("returns typed 403 when optional required scopes are missing", func(t *testing.T) {
		scopes := []string{permissions.FileCreate}
		resp, introErr := adminClient.IntrospectOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			ct.IntrospectOrganizationAPIKeyJSONRequestBody{ApiKey: clearKey, RequiredScopes: &scopes},
		)
		require.NoError(t, introErr)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
		require.NotNil(t, resp.JSON403)

		assert.Equal(t, org.Id, resp.JSON403.ApiKey.OrganizationId)
		assert.ElementsMatch(t, []string{permissions.FileRead}, resp.JSON403.Permissions)
		assert.ElementsMatch(t, []string{permissions.FileCreate}, resp.JSON403.MissingPrivileges)
	})

	t.Run("returns 404 for an unknown key", func(t *testing.T) {
		// The caller is authenticated; an unknown subject credential is a
		// not-found, not a caller-authentication failure.
		resp, introErr := adminClient.IntrospectOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			ct.IntrospectOrganizationAPIKeyJSONRequestBody{ApiKey: "nanostack_org_apikey_not_a_real_key"},
		)
		require.NoError(t, introErr)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

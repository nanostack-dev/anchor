package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	itshared "anchor/cmd/it/shared"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationAPIKeyValidate(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	adminClient, _ := product.CreateAPIKeyClientWithScopes([]string{
		"organization_api_key:create",
		"organization_api_key:read",
	})

	permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
	description := itshared.Faker.Lorem().Sentence(4)
	org := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description)
	apiKeyName := "OrgKey-" + uuid.NewString()

	createResp, err := adminClient.CreateOrganizationAPIKeyWithResponse(
		ctx,
		product.ProductID,
		org.Id,
		ct.CreateOrganizationAPIKeyJSONRequestBody{
			Name:        apiKeyName,
			Permissions: []string{permissions.FileRead},
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)
	require.NotEmpty(t, createResp.JSON201.Value)

	clearKey := createResp.JSON201.Value

	t.Run("returns 200 when required scopes are satisfied", func(t *testing.T) {
		resp, validateErr := adminClient.ValidateOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			ct.ValidateOrganizationAPIKeyJSONRequestBody{
				ApiKey:         clearKey,
				RequiredScopes: []string{permissions.FileRead},
			},
		)
		require.NoError(t, validateErr)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)

		assert.Equal(t, createResp.JSON201.Id, resp.JSON200.ApiKey.Id)
		assert.Equal(t, org.Id, resp.JSON200.ApiKey.OrganizationId)
		assert.ElementsMatch(t, []string{permissions.FileRead}, resp.JSON200.Permissions)
		assert.Empty(t, resp.JSON200.MissingPrivileges)
	})

	t.Run("returns typed 403 when scopes are missing", func(t *testing.T) {
		resp, validateErr := adminClient.ValidateOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			ct.ValidateOrganizationAPIKeyJSONRequestBody{
				ApiKey:         clearKey,
				RequiredScopes: []string{permissions.FileCreate},
			},
		)
		require.NoError(t, validateErr)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
		require.NotNil(t, resp.JSON403)

		assert.Equal(t, createResp.JSON201.Id, resp.JSON403.ApiKey.Id)
		assert.Equal(t, org.Id, resp.JSON403.ApiKey.OrganizationId)
		assert.ElementsMatch(t, []string{permissions.FileRead}, resp.JSON403.Permissions)
		assert.ElementsMatch(t, []string{permissions.FileCreate}, resp.JSON403.MissingPrivileges)
	})

	t.Run("returns typed 403 when api key is expired", func(t *testing.T) {
		expiredAt := time.Now().UTC().Add(1 * time.Second).Truncate(time.Second)
		expiredCreateResp, createErr := adminClient.CreateOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			ct.CreateOrganizationAPIKeyJSONRequestBody{
				Name:        "Expired-" + uuid.NewString(),
				ExpiresAt:   &expiredAt,
				Permissions: []string{permissions.FileRead},
			},
		)
		require.NoError(t, createErr)
		require.Equal(t, http.StatusCreated, expiredCreateResp.StatusCode())
		require.NotNil(t, expiredCreateResp.JSON201)

		time.Sleep(2 * time.Second)

		var validateResp *ct.ValidateOrganizationAPIKeyResponse
		require.Eventually(t, func() bool {
			resp, validateErr := adminClient.ValidateOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				org.Id,
				ct.ValidateOrganizationAPIKeyJSONRequestBody{
					ApiKey:         expiredCreateResp.JSON201.Value,
					RequiredScopes: []string{permissions.FileRead},
				},
			)
			if validateErr != nil {
				return false
			}
			validateResp = resp
			return resp.StatusCode() == http.StatusForbidden &&
				resp.JSON403 != nil &&
				resp.JSON403.ApiKey.Status == ct.OrganizationAPIKeyStatusINACTIVE
		}, 10*time.Second, 500*time.Millisecond)

		require.NotNil(t, validateResp)
		require.Equal(t, http.StatusForbidden, validateResp.StatusCode())
		require.NotNil(t, validateResp.JSON403)
		assert.Equal(t, expiredCreateResp.JSON201.Id, validateResp.JSON403.ApiKey.Id)
		assert.ElementsMatch(t, []string{permissions.FileRead}, validateResp.JSON403.Permissions)
		assert.Empty(t, validateResp.JSON403.MissingPrivileges)
		assert.Equal(t, ct.OrganizationAPIKeyStatusINACTIVE, validateResp.JSON403.ApiKey.Status)
	})
}

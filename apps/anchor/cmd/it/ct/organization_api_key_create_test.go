package ct_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	itshared "anchor/cmd/it/shared"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationAPIKeyCreate(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	apiKeyClient, _ := product.CreateAPIKeyClientWithScopes(
		[]string{
			"organization_api_key:create",
			"organization_api_key:read",
		},
	)
	permissions := givenOrganizationAPIKeyResourcePermissions(t, product)

	t.Run(
		"Create organization API key", func(t *testing.T) {
			description := itshared.Faker.Lorem().Sentence(4)
			org := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description)
			apiKeyName := "OrgKey-" + uuid.NewString()
			apiKeyDescription := itshared.Faker.Lorem().Sentence(5)
			expiresAt := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)

			resp, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				org.Id,
				ct.CreateOrganizationAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Description: &apiKeyDescription,
					ExpiresAt:   &expiresAt,
					Permissions: []string{permissions.FileRead, permissions.FileCreate},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, resp.StatusCode())
			require.NotNil(t, resp.JSON201)

			created := resp.JSON201
			assert.NotEmpty(t, created.Id)
			assert.Equal(t, org.Id, created.OrganizationId)
			assert.Equal(t, apiKeyName, created.Name)
			assert.Equal(t, apiKeyDescription, *created.Description)
			assert.NotEmpty(t, created.Value)
			assert.NotEqual(t, created.Value, created.ObfuscatedValue)
			require.NotNil(t, created.ExpiresAt)
			assert.WithinDuration(t, expiresAt, *created.ExpiresAt, time.Second)
			assert.Equal(t, ct.OrganizationAPIKeyStatusACTIVE, created.Status)
			require.Len(t, created.Permissions, 2)
			assert.Equal(t, permissions.FileRead, created.Permissions[0].PermissionName)
			assert.Equal(t, product.ProductID, created.Permissions[0].ProductId)
			assert.Equal(t, org.Id, created.Permissions[0].OrganizationId)
		},
	)

	t.Run(
		"Create organization API key uses product API key prefix", func(t *testing.T) {
			prefix := "acmeorg"
			getProductResp, err := product.OwnerAuthenticatedClient().GetProductWithResponse(
				ctx,
				product.ProductID,
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, getProductResp.StatusCode())
			require.NotNil(t, getProductResp.JSON200)

			updateProductResp, err := product.OwnerAuthenticatedClient().UpdateProductWithResponse(
				ctx,
				product.ProductID,
				ct.UpdateProductJSONRequestBody{
					Name:        getProductResp.JSON200.Name,
					Description: getProductResp.JSON200.Description,
					Config: &ct.ProductConfigRequest{ApiKeys: &ct.ProductAPIKeysConfigRequest{
						Prefix: prefix,
					}},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, updateProductResp.StatusCode())

			description := itshared.Faker.Lorem().Sentence(4)
			org := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description)
			resp, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				org.Id,
				ct.CreateOrganizationAPIKeyJSONRequestBody{
					Name:        "OrgKey-" + uuid.NewString(),
					Permissions: []string{permissions.FileRead},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, resp.StatusCode())
			require.NotNil(t, resp.JSON201)
			assert.True(t, strings.HasPrefix(resp.JSON201.Value, prefix+"_org_apikey_"))
			assert.True(t, strings.HasPrefix(resp.JSON201.ObfuscatedValue, prefix+"_org_apikey_"))
		},
	)

	t.Run(
		"Create organization API key without permissions returns bad request", func(t *testing.T) {
			description := itshared.Faker.Lorem().Sentence(4)
			org := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description)

			// Org API key permissions are immutable after creation, so at least
			// one must be supplied; an empty/omitted list is rejected.
			resp, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				org.Id,
				ct.CreateOrganizationAPIKeyJSONRequestBody{
					Name:        "OrgKeyNoPerms-" + uuid.NewString(),
					Permissions: []string{},
				},
			)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
			require.NotNil(t, resp.JSON400)
			assert.Contains(t, resp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
		},
	)

	t.Run(
		"Duplicate name in same organization returns bad request", func(t *testing.T) {
			description := itshared.Faker.Lorem().Sentence(4)
			org := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description)
			name := "DupKey-" + uuid.NewString()

			firstResp, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				org.Id,
				ct.CreateOrganizationAPIKeyJSONRequestBody{
					Name:        name,
					Permissions: []string{permissions.FileRead},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, firstResp.StatusCode())

			dupResp, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				org.Id,
				ct.CreateOrganizationAPIKeyJSONRequestBody{
					Name:        name,
					Permissions: []string{permissions.FileRead},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, dupResp.StatusCode())
			if assert.NotNil(t, dupResp.JSON400) {
				AssertAPIError(
					t,
					dupResp.JSON400.Errors,
					ct.ApiError{
						Code:    "ORGANIZATION_API_KEY_NAME_DUPLICATE",
						Message: "Organization API key with this name already exists in the organization",
					},
				)
			}
		},
	)

	t.Run(
		"Create organization API key with past expiration returns bad request", func(t *testing.T) {
			description := itshared.Faker.Lorem().Sentence(4)
			org := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description)
			expiresAt := time.Now().UTC().Add(-1 * time.Minute).Truncate(time.Second)

			resp, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				org.Id,
				ct.CreateOrganizationAPIKeyJSONRequestBody{
					Name:        "PastExpiry-" + uuid.NewString(),
					ExpiresAt:   &expiresAt,
					Permissions: []string{permissions.FileRead},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode())
			if assert.NotNil(t, resp.JSON400) {
				AssertAPIError(
					t,
					resp.JSON400.Errors,
					ct.ApiError{
						Code:    "ORGANIZATION_API_KEY_EXPIRES_AT_IN_PAST",
						Message: "Organization API key expiration date must be in the future",
					},
				)
			}
		},
	)

	t.Run(
		"Same name in different organizations is allowed", func(t *testing.T) {
			description1 := itshared.Faker.Lorem().Sentence(4)
			description2 := itshared.Faker.Lorem().Sentence(4)
			org1 := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description1)
			org2 := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description2)
			name := "SharedName-" + uuid.NewString()

			resp1, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				org1.Id,
				ct.CreateOrganizationAPIKeyJSONRequestBody{
					Name:        name,
					Permissions: []string{permissions.FileRead},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, resp1.StatusCode())

			resp2, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				org2.Id,
				ct.CreateOrganizationAPIKeyJSONRequestBody{
					Name:        name,
					Permissions: []string{permissions.FileRead},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, resp2.StatusCode())
		},
	)

	t.Run(
		"Cross product access is denied", func(t *testing.T) {
			otherProduct := createTestProductContext(t)
			description := itshared.Faker.Lorem().Sentence(4)
			otherOrg := otherProduct.CreateOrganization(t, "Org-"+uuid.NewString(), &description)

			resp, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				otherOrg.Id,
				ct.CreateOrganizationAPIKeyJSONRequestBody{
					Name:        "CrossProduct-" + uuid.NewString(),
					Permissions: []string{permissions.FileRead},
				},
			)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, resp.StatusCode())
		},
	)

	t.Run(
		"Create organization API key with Near expiration", func(t *testing.T) {
			description := itshared.Faker.Lorem().Sentence(4)
			org := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description)
			apiKeyName := "OrgKey-" + uuid.NewString()
			apiKeyDescription := itshared.Faker.Lorem().Sentence(5)
			expiresAt := time.Now().UTC().Add(5 * time.Second).Truncate(time.Second)
			response, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				org.Id,
				ct.CreateOrganizationAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Description: &apiKeyDescription,
					ExpiresAt:   &expiresAt,
					Permissions: []string{permissions.FileRead, permissions.FileCreate},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, response.StatusCode())
			require.NotNil(t, response.JSON201)
			require.Equal(t, ct.OrganizationAPIKeyStatusACTIVE, response.JSON201.Status)
			t.Log("Waiting for API key to expire...")
			assert.Eventually(t, func() bool {
				apiKeyExpired, apiKeyGetErr := apiKeyClient.GetOrganizationAPIKeyWithResponse(
					ctx, product.ProductID, org.Id, response.JSON201.Id,
				)
				if apiKeyGetErr != nil || apiKeyExpired == nil || apiKeyExpired.JSON200 == nil {
					return false
				}
				return apiKeyExpired.JSON200.Status == ct.OrganizationAPIKeyStatusINACTIVE
			}, 15*time.Second, 500*time.Millisecond)
		},
	)
}

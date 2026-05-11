package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/nanostack-dev/shared/toolkit"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductAPIKeyGet(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	permission1 := "organization:read"
	permission2 := "organization:create"

	apiKeyName := "TestGetKey_" + uuid.NewString()
	description := "Test API key for get operations"

	createResp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
		ctx, product.ProductID,
		ct.CreateProductAPIKeyJSONRequestBody{
			Name:        apiKeyName,
			Description: &description,
			Permissions: []string{permission1, permission2},
		},
	)
	require.NoError(t, err)
	require.Equal(
		t, http.StatusCreated,
		createResp.StatusCode(),
	)
	require.NotNil(t, createResp.JSON201)

	apiKeyID := createResp.JSON201.Id

	t.Run(
		"Get existing API key", func(t *testing.T) {
			getResp, getErr := product.OwnerAuthenticatedClient().GetProductAPIKeyWithResponse(
				ctx, product.ProductID, apiKeyID,
			)
			require.NoError(t, getErr)
			assert.Equal(t, 200, getResp.StatusCode())
			assert.NotNil(t, getResp.JSON200)

			apiKey := getResp.JSON200
			assert.Equal(t, apiKeyID, apiKey.Id)
			assert.Equal(t, product.ProductID, apiKey.ProductId)
			assert.Equal(t, apiKeyName, apiKey.Name)
			assert.Equal(t, description, *apiKey.Description)
			assert.Equal(t, ct.ProductAPIKeyStatusACTIVE, apiKey.Status)
			assert.NotEmpty(t, apiKey.ObfuscatedValue)
			assert.NotEmpty(t, apiKey.CreatedAt)
			assert.NotEmpty(t, apiKey.UpdatedAt)
			assert.Len(t, apiKey.Permissions, 2)

			permissionNames := make(map[string]bool)
			for _, perm := range apiKey.Permissions {
				permissionNames[perm.PermissionName] = true
				assert.Equal(t, product.ProductID, perm.ProductId)
				assert.Equal(t, apiKeyID, perm.ProductApiKeyId)
			}
			assert.True(t, permissionNames[permission1])
			assert.True(t, permissionNames[permission2])
		},
	)

	t.Run(
		"Get API key validation scenarios", func(t *testing.T) {
			testCases := []struct {
				name           string
				productID      string
				apiKeyID       string
				expectedStatus int
			}{
				{
					name:           "Valid product and API key IDs",
					productID:      product.ProductID,
					apiKeyID:       apiKeyID,
					expectedStatus: 200,
				},
				{
					name:           "Invalid product ID",
					productID:      toolkit.NewID("prd"),
					apiKeyID:       apiKeyID,
					expectedStatus: 404,
				},
				{
					name:           "Invalid API key ID",
					productID:      product.ProductID,
					apiKeyID:       toolkit.NewID("product_apikey"),
					expectedStatus: 404,
				},
				{
					name:           "Both IDs invalid",
					productID:      toolkit.NewID("prd"),
					apiKeyID:       toolkit.NewID("product_apikey"),
					expectedStatus: 404,
				},
			}

			for _, tc := range testCases {
				t.Run(
					tc.name, func(t *testing.T) {
						getResp, testErr := product.OwnerAuthenticatedClient().GetProductAPIKeyWithResponse(
							ctx, tc.productID, tc.apiKeyID,
						)
						require.NoError(t, testErr)
						assert.Equal(t, tc.expectedStatus, getResp.StatusCode())

						if tc.expectedStatus == 200 {
							assert.NotNil(t, getResp.JSON200)
							assert.Equal(t, tc.apiKeyID, getResp.JSON200.Id)
							assert.Equal(t, tc.productID, getResp.JSON200.ProductId)
						} else {
							assert.Nil(t, getResp.JSON200)
						}
					},
				)
			}
		},
	)
}

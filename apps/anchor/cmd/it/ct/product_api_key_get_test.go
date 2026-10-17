package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductAPIKeyGet(t *testing.T) {
	// Checks product API key lookup across the scenarios below.
	t.Parallel()
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
			// Reading existing API key succeeds.
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
			// Get API key validation scenarios.
			testCases := []struct {
				name           string
				productID      string
				apiKeyID       string
				expectedStatus int
			}{
				// The product-key operation accepts valid product and API key IDs.
				{
					name:           "Valid product and API key IDs",
					productID:      product.ProductID,
					apiKeyID:       apiKeyID,
					expectedStatus: 200,
				},
				// The product-key operation refuses invalid product ID.
				{
					name:           "Invalid product ID",
					productID:      ids.MustNew("prd"),
					apiKeyID:       apiKeyID,
					expectedStatus: 404,
				},
				// The product-key operation refuses invalid API key ID.
				{
					name:           "Invalid API key ID",
					productID:      product.ProductID,
					apiKeyID:       ids.MustNew("product_apikey"),
					expectedStatus: 404,
				},
				// The product-key operation refuses both IDs invalid.
				{
					name:           "Both IDs invalid",
					productID:      ids.MustNew("prd"),
					apiKeyID:       ids.MustNew("product_apikey"),
					expectedStatus: 404,
				},
			}

			for _, tc := range testCases {
				t.Run(
					tc.name, func(t *testing.T) {
						// Product-key lookup returns the expected result for this product and key identifier pair.
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

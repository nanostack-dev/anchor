package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductAPIKeyDelete(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	permission1 := "organization:read"

	t.Run(
		"Delete existing API key successfully", func(t *testing.T) {
			apiKeyName := "DeleteKey_" + uuid.NewString()

			createResp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Description: ptr.Ptr("Key to be deleted"),
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)
			require.NotNil(t, createResp.JSON201)

			apiKeyID := createResp.JSON201.Id

			deleteResp, err := product.OwnerAuthenticatedClient().DeleteProductAPIKeyWithResponse(
				ctx, product.ProductID, apiKeyID,
			)
			require.NoError(t, err)
			assert.Equal(t, 204, deleteResp.StatusCode())

			getResp, err := product.OwnerAuthenticatedClient().GetProductAPIKeyWithResponse(
				ctx, product.ProductID, apiKeyID,
			)
			require.NoError(t, err)
			assert.Equal(t, 404, getResp.StatusCode())
			assert.Nil(t, getResp.JSON200)
		},
	)

	t.Run(
		"Delete API key table driven scenarios", func(t *testing.T) {
			validAPIKeyName := "ValidDeleteKey_" + uuid.NewString()

			createResp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        validAPIKeyName,
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)
			require.NotNil(t, createResp.JSON201)

			testCases := []struct {
				name           string
				productID      string
				apiKeyID       string
				expectedStatus int
			}{
				{
					name:           "Valid product and API key IDs",
					productID:      product.ProductID,
					apiKeyID:       createResp.JSON201.Id,
					expectedStatus: 204,
				},
				{
					name:           "Invalid product ID",
					productID:      ids.MustNew("prd"),
					apiKeyID:       createResp.JSON201.Id,
					expectedStatus: 404,
				},
				{
					name:           "Invalid API key ID",
					productID:      product.ProductID,
					apiKeyID:       ids.MustNew("product_apikey"),
					expectedStatus: 404,
				},
			}

			for _, tc := range testCases {
				t.Run(
					tc.name, func(t *testing.T) {
						deleteResp, deleteErr := product.OwnerAuthenticatedClient().DeleteProductAPIKeyWithResponse(
							ctx, tc.productID, tc.apiKeyID,
						)
						require.NoError(t, deleteErr)
						assert.Equal(t, tc.expectedStatus, deleteResp.StatusCode())
					},
				)
			}
		},
	)

	t.Run(
		"Delete API key and verify it cannot be retrieved", func(t *testing.T) {
			apiKeyName := "VerifyDeleteKey_" + uuid.NewString()

			createResp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Description: ptr.Ptr("Key for verification test"),
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)
			require.NotNil(t, createResp.JSON201)

			apiKeyID := createResp.JSON201.Id

			getResp1, err := product.OwnerAuthenticatedClient().GetProductAPIKeyWithResponse(
				ctx, product.ProductID, apiKeyID,
			)
			require.NoError(t, err)
			assert.Equal(t, 200, getResp1.StatusCode())
			assert.NotNil(t, getResp1.JSON200)

			deleteResp, err := product.OwnerAuthenticatedClient().DeleteProductAPIKeyWithResponse(
				ctx, product.ProductID, apiKeyID,
			)
			require.NoError(t, err)
			assert.Equal(t, 204, deleteResp.StatusCode())

			getResp2, err := product.OwnerAuthenticatedClient().GetProductAPIKeyWithResponse(
				ctx, product.ProductID, apiKeyID,
			)
			require.NoError(t, err)
			assert.Equal(t, 404, getResp2.StatusCode())
			assert.Nil(t, getResp2.JSON200)

			searchResp, err := product.OwnerAuthenticatedClient().SearchProductAPIKeysWithResponse(
				ctx, product.ProductID, ct.SearchProductAPIKeysJSONRequestBody{
					Filter: &ct.ProductAPIKeyFilter{
						Ids: []string{apiKeyID},
					},
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.Empty(t, searchResp.JSON200.Items)
			assert.Equal(t, int64(0), searchResp.JSON200.Total)
		},
	)

	t.Run(
		"If multiple api keys are created, deleting one should not affect others",
		func(t *testing.T) {
			apiKeyName1 := "MultiDeleteKey1_" + uuid.NewString()
			apiKeyName2 := "MultiDeleteKey2_" + uuid.NewString()

			createResp1, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName1,
					Description: ptr.Ptr("First key for multi-delete test"),
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp1.StatusCode(),
			)
			require.NotNil(t, createResp1.JSON201)

			createResp2, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName2,
					Description: ptr.Ptr("Second key for multi-delete test"),
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp2.StatusCode(),
			)
			require.NotNil(t, createResp2.JSON201)

			apiKeyID1 := createResp1.JSON201.Id
			apiKeyID2 := createResp2.JSON201.Id

			deleteResp1, err := product.OwnerAuthenticatedClient().DeleteProductAPIKeyWithResponse(
				ctx, product.ProductID, apiKeyID1,
			)
			require.NoError(t, err)
			assert.Equal(t, 204, deleteResp1.StatusCode())

			getResp1, err := product.OwnerAuthenticatedClient().GetProductAPIKeyWithResponse(
				ctx, product.ProductID, apiKeyID1,
			)
			require.NoError(t, err)
			assert.Equal(t, 404, getResp1.StatusCode())

			getResp2, err := product.OwnerAuthenticatedClient().GetProductAPIKeyWithResponse(
				ctx, product.ProductID, apiKeyID2,
			)
			require.NoError(t, err)
			assert.Equal(t, 200, getResp2.StatusCode())

			assert.NotNil(t, getResp2.JSON200)
		},
	)
}

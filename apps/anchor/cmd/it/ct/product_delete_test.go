package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/nanostack-dev/shared/toolkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductDelete(t *testing.T) {
	ctx := t.Context()

	t.Run(
		"BasicDeleteProduct", func(t *testing.T) {
			// Create a test product first
			createResp, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        "Test Product for Deletion",
					Description: toolkit.Ptr("This is a test product"),
				},
			)
			require.NoError(t, err, "create product request should not error")
			assert.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(), "create product should return 201 Created",
			)
			assert.NotNil(t, createResp.JSON201)

			productID := createResp.JSON201.Id

			// Now delete the created product
			deleteResp, err := testOwnerClient(t).DeleteProductWithResponse(
				ctx, productID,
			)
			require.NoError(t, err, "delete product request should not error")
			assert.Equal(
				t, 204, deleteResp.StatusCode(), "delete product should return 204 No Content",
			)
		},
	)

	t.Run(
		"DeleteNonExistentProduct", func(t *testing.T) {
			nonExistentProductID := toolkit.NewID("prd")

			resp, err := testOwnerClient(t).DeleteProductWithResponse(
				ctx, nonExistentProductID,
			)
			require.NoError(t, err, "delete non-existent product request should not error")
			assert.Equal(
				t, 404, resp.StatusCode(),
				"delete non-existent product should return 404 No Content",
			)
		},
	)

	t.Run(
		"DeleteAProductAlsoDeletesItsAssociatedData", func(t *testing.T) {
			// Create a test product first
			createResp, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        "Test Product for Deletion with Associated Data",
					Description: toolkit.Ptr("This is a test product"),
				},
			)
			require.NoError(t, err, "create product request should not error")
			assert.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(), "create product should return 201 Created",
			)
			assert.NotNil(t, createResp.JSON201)

			productID := createResp.JSON201.Id

			// Now delete the created product
			deleteResp, err := testOwnerClient(t).DeleteProductWithResponse(
				ctx, productID,
			)
			require.NoError(t, err, "delete product request should not error")
			assert.Equal(
				t, 204, deleteResp.StatusCode(), "delete product should return 204 No Content",
			)
			//TODO: check that associated data is also deleted
		},
	)
}

// filepath: ~/Documents/NanostackProject/anchor/test/ct/product_update_test.go
package ct_test

import (
	"net/http"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductUpdate(t *testing.T) {
	ctx := t.Context()

	t.Run(
		"SuccessfulUpdateProduct", func(t *testing.T) {
			productName := "Test Product For Update"
			createResp, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        productName,
					Description: ptr.Ptr("This is a test product"),
				},
			)
			require.NoError(t, err, "create product request should not error")
			assert.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(), "create product should return 201 Created",
			)
			assert.NotNil(t, createResp.JSON201)

			productID := createResp.JSON201.Id

			// Update the product
			updatedName := "Updated Product Name"
			updatedDesc := "This is an updated description"
			updateResp, err := testOwnerClient(t).UpdateProductWithResponse(
				ctx,
				productID,
				ct.UpdateProductJSONRequestBody{
					Name:        updatedName,
					Description: ptr.Ptr(updatedDesc),
				},
			)
			require.NoError(t, err, "update product request should not error")
			assert.Equal(t, 200, updateResp.StatusCode(), "update product should return 200 OK")
			assert.NotNil(t, updateResp.JSON200)
			assert.Equal(t, updatedName, updateResp.JSON200.Name, "product name should be updated")
			assert.Equal(
				t, updatedDesc, *updateResp.JSON200.Description,
				"product description should be updated",
			)
		},
	)

	t.Run(
		"UpdateProductWithEmptyName", func(t *testing.T) {
			productName := "Test Product Empty Name"
			createResp, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        productName,
					Description: ptr.Ptr("This is a test product"),
				},
			)
			require.NoError(t, err, "create product request should not error")
			assert.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(), "create product should return 201 Created",
			)
			assert.NotNil(t, createResp.JSON201)

			productID := createResp.JSON201.Id

			updateResp, err := testOwnerClient(t).UpdateProductWithResponse(
				ctx,
				productID,
				ct.UpdateProductJSONRequestBody{
					Name:        "",
					Description: ptr.Ptr("Updated description"),
				},
			)
			require.NoError(t, err, "update product with empty name should not error")
			assert.NotNil(t, updateResp)
			assert.Equal(
				t, 400, updateResp.StatusCode(),
				"update product with empty name should return 400 Bad Request",
			)
			assert.Contains(t, updateResp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
			assert.Contains(t, updateResp.JSON400.Errors[0].Message, "name cannot be blank")
		},
	)

	t.Run(
		"UpdateProductWithInvalidDescription", func(t *testing.T) {
			productName := "Test Product Invalid Description"
			createResp, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        productName,
					Description: ptr.Ptr("This is a test product"),
				},
			)
			require.NoError(t, err, "create product request should not error")
			assert.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(), "create product should return 201 Created",
			)
			assert.NotNil(t, createResp.JSON201)

			productID := createResp.JSON201.Id

			updateResp, err := testOwnerClient(t).UpdateProductWithResponse(
				ctx,
				productID,
				ct.UpdateProductJSONRequestBody{
					Name:        "Updated Name",
					Description: ptr.Ptr(strings.Repeat("a", 1001)),
				},
			)
			require.NoError(t, err, "update product with invalid description should not error")
			assert.Equal(
				t, 400, updateResp.StatusCode(),
				"update product with invalid description should return 400 Bad Request",
			)
			assert.Contains(t, updateResp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
			assert.Contains(
				t, updateResp.JSON400.Errors[0].Message,
				"description must be a maximum of 1,000 characters in length",
			)
		},
	)

	t.Run(
		"UpdateNonExistentProduct", func(t *testing.T) {
			nonExistentProductID := ids.MustNew("prd")

			updateResp, err := testOwnerClient(t).UpdateProductWithResponse(
				ctx,
				nonExistentProductID,
				ct.UpdateProductJSONRequestBody{
					Name:        "Updated Name",
					Description: ptr.Ptr("Updated description"),
				},
			)
			require.NoError(t, err, "update non-existent product request should not error")
			assert.Equal(
				t, 404, updateResp.StatusCode(),
				"update non-existent product should return 404 Not Found",
			)
		},
	)

	t.Run(
		"UpdateProductWithDuplicateName", func(t *testing.T) {
			// Create two test products
			product1Name := "Product One"
			createResp1, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        product1Name,
					Description: ptr.Ptr("This is the first test product"),
				},
			)
			require.NoError(t, err, "create first product request should not error")
			assert.Equal(
				t, http.StatusCreated,
				createResp1.StatusCode(), "create product should return 201 Created",
			)

			product2Name := "Product Two"
			createResp2, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        product2Name,
					Description: ptr.Ptr("This is the second test product"),
				},
			)
			require.NoError(t, err, "create second product request should not error")
			assert.Equal(
				t, http.StatusCreated,
				createResp2.StatusCode(), "create product should return 201 Created",
			)

			updateResp, err := testOwnerClient(t).UpdateProductWithResponse(
				ctx,
				createResp2.JSON201.Id,
				ct.UpdateProductJSONRequestBody{
					Name:        product1Name,
					Description: ptr.Ptr("Attempting to use duplicate name"),
				},
			)
			require.NoError(t, err, "update product with duplicate name should not error")
			assert.Equal(
				t, 400, updateResp.StatusCode(),
				"update product with duplicate name should return 400 Bad Request",
			)
			assert.Contains(t, updateResp.JSON400.Errors[0].Code, "PRODUCT_ALREADY_EXISTS")
		},
	)
}

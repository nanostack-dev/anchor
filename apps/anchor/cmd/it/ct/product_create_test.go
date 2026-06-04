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

func TestProductCreate(t *testing.T) {
	ctx := t.Context()

	t.Run(
		"SuccessfulCreateProduct", func(t *testing.T) {
			response, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        "Test Product",
					Description: ptr.Ptr("This is a test product"),
				},
			)
			assert.NotNil(t, response, "response should not be nil")
			require.NoError(t, err, "create product request should not error")
			assert.Equal(
				t, http.StatusCreated,
				response.StatusCode(), "create product should return 201 Created",
			)
		},
	)

	t.Run(
		"CreateProductWithEmptyName", func(t *testing.T) {
			response, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        "",
					Description: ptr.Ptr("This product has no name"),
				},
			)
			require.NoError(t, err, "create product with empty name should not error")
			assert.NotNil(t, response, "response should not be nil")
			assert.NotNil(t, response.JSON400, "response should not be nil")
			assert.Equal(
				t, 400, response.StatusCode(),
				"create product with empty name should return 400 Bad Request",
			)
			assert.Contains(t, response.JSON400.Errors[0].Code, "VALIDATION_ERROR")
			assert.Contains(t, response.JSON400.Errors[0].Message, "name is a required field")
		},
	)

	t.Run(
		"CreateProductWithInvalidDescription", func(t *testing.T) {
			response, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        "Invalid Description Product",
					Description: ptr.Ptr(strings.Repeat("a", 1001)),
				},
			)
			assert.NotNil(t, response, "response should not be nil")
			require.NoError(t, err, "create product with invalid description should not error")
			assert.Equal(
				t, 400, response.StatusCode(),
				"create product with invalid description should return 400 Bad Request",
			)
			assert.Contains(t, response.JSON400.Errors[0].Code, "VALIDATION_ERROR")
			assert.Contains(
				t, response.JSON400.Errors[0].Message,
				"description must be a maximum of 1,000 characters in length",
			)
		},
	)

	t.Run(
		"CreateProductWithDuplicateName", func(t *testing.T) {
			createResp, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        "Duplicate Product",
					Description: ptr.Ptr("This is a test product"),
				},
			)
			require.NoError(t, err, "create product request should not error")
			assert.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(), "create product should return 201 Created",
			)

			// Now try to create another product with the same name
			duplicateResp, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        "Duplicate Product",
					Description: ptr.Ptr("This is another test product"),
				},
			)
			assert.NotNil(t, duplicateResp, "response should not be nil")
			require.NoError(t, err, "create duplicate product request should not error")
			assert.Equal(
				t, 400, duplicateResp.StatusCode(),
				"create duplicate product should return 400 Bad Request",
			)
			assert.Contains(t, duplicateResp.JSON400.Errors[0].Code, "PRODUCT_ALREADY_EXISTS")
			assert.Contains(
				t, duplicateResp.JSON400.Errors[0].Message,
				"A product with this name already exists in your tenant",
			)
		},
	)

	t.Run(
		"CreateProductWithDuplicateNameDifferentCase", func(t *testing.T) {
			productName := "Duplicate Case Product " + ids.MustNew("test")

			createResp, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        productName,
					Description: ptr.Ptr("This is a test product"),
				},
			)
			require.NoError(t, err, "create product request should not error")
			assert.Equal(t, http.StatusCreated, createResp.StatusCode())

			duplicateResp, err := testOwnerClient(t).CreateProductWithResponse(
				ctx,
				ct.CreateProductJSONRequestBody{
					Name:        strings.ToLower(productName),
					Description: ptr.Ptr("This is another test product"),
				},
			)
			require.NoError(t, err, "create duplicate product request should not error")
			assert.Equal(t, http.StatusBadRequest, duplicateResp.StatusCode())
			assert.Contains(t, duplicateResp.JSON400.Errors[0].Code, "PRODUCT_ALREADY_EXISTS")
		},
	)
}

package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductOrganizationUpdate(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	apiKeyClient, _ := testProduct.CreateAPIKeyClientWithAllScopes()

	t.Run(
		"SuccessfulUpdateOrganization", func(t *testing.T) {
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Original Organization",
					Description: ptr.Ptr("Original description"),
				},
			)

			require.NoError(t, err, "create organization should not error")
			require.NotNil(t, createResponse.JSON201, "create response should not be nil")
			organizationID := createResponse.JSON201.Id

			updateResponse, err := apiKeyClient.UpdateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
				ct.UpdateProductOrganizationJSONRequestBody{
					Name:        "Updated Organization",
					Description: ptr.Ptr("Updated description"),
				},
			)

			require.NoError(t, err, "update organization request should not error")
			assert.NotNil(t, updateResponse, "response should not be nil")

			if assert.NotNil(t, updateResponse.JSON200, "response body should not be nil") {
				assert.Equal(
					t, testProduct.ProductID, updateResponse.JSON200.ProductId,
					"product ID should match",
				)
				assert.Equal(
					t, organizationID, updateResponse.JSON200.Id, "organization ID should match",
				)
				assert.Equal(
					t, "Updated Organization", updateResponse.JSON200.Name,
					"organization name should be updated",
				)
				assert.Equal(
					t, "Updated description", *updateResponse.JSON200.Description,
					"description should be updated",
				)
				assert.NotEmpty(
					t, updateResponse.JSON200.CreatedAt, "created at should not be empty",
				)
				assert.NotEmpty(
					t, updateResponse.JSON200.UpdatedAt, "updated at should not be empty",
				)
				assert.NotEqual(
					t, createResponse.JSON201.UpdatedAt, updateResponse.JSON200.UpdatedAt,
					"updated at should change after update",
				)
			}
		},
	)

	t.Run(
		"UpdateOrganizationNameOnly", func(t *testing.T) {
			// Create an organization to update
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Name Only Test",
					Description: ptr.Ptr("Original description"),
				},
			)

			require.NoError(t, err, "create organization should not error")
			require.NotNil(t, createResponse.JSON201, "create response should not be nil")
			organizationID := createResponse.JSON201.Id

			// Update only the name
			updateResponse, err := apiKeyClient.UpdateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
				ct.UpdateProductOrganizationJSONRequestBody{
					Name:        "Updated Name Only",
					Description: ptr.Ptr("Original description"), // Keep same description
				},
			)

			require.NoError(t, err, "update organization name should not error")
			assert.Equal(t, 200, updateResponse.StatusCode(), "should return 200 OK")

			if assert.NotNil(t, updateResponse.JSON200, "response body should not be nil") {
				assert.Equal(
					t, "Updated Name Only", updateResponse.JSON200.Name, "name should be updated",
				)
				assert.Equal(
					t, "Original description", *updateResponse.JSON200.Description,
					"description should remain unchanged",
				)
			}
		},
	)

	t.Run(
		"UpdateOrganizationDescriptionOnly", func(t *testing.T) {
			// Create an organization to update
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Description Only Test",
					Description: ptr.Ptr("Original description"),
				},
			)

			require.NoError(t, err, "create organization should not error")
			require.NotNil(t, createResponse.JSON201, "create response should not be nil")
			organizationID := createResponse.JSON201.Id

			// Update only the description
			updateResponse, err := apiKeyClient.UpdateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
				ct.UpdateProductOrganizationJSONRequestBody{
					Name:        "Description Only Test", // Keep same name
					Description: ptr.Ptr("Updated description only"),
				},
			)

			require.NoError(t, err, "update organization description should not error")
			assert.Equal(t, 200, updateResponse.StatusCode(), "should return 200 OK")

			if assert.NotNil(t, updateResponse.JSON200, "response body should not be nil") {
				assert.Equal(
					t, "Description Only Test", updateResponse.JSON200.Name,
					"name should remain unchanged",
				)
				assert.Equal(
					t, "Updated description only", *updateResponse.JSON200.Description,
					"description should be updated",
				)
			}
		},
	)

	t.Run(
		"UpdateOrganizationWithEmptyName", func(t *testing.T) {
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Valid Organization",
					Description: ptr.Ptr("Valid description"),
				},
			)

			require.NoError(t, err, "create organization should not error")
			require.NotNil(t, createResponse.JSON201, "create response should not be nil")
			organizationID := createResponse.JSON201.Id

			updateResponse, err := apiKeyClient.UpdateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
				ct.UpdateProductOrganizationJSONRequestBody{
					Name:        "",
					Description: ptr.Ptr("Valid description"),
				},
			)

			require.NoError(t, err, "update organization with empty name should not error")
			assert.NotNil(t, updateResponse, "response should not be nil")
			assert.Equal(
				t, http.StatusBadRequest, updateResponse.StatusCode(),
				"update organization with empty name should return 400 Bad Request",
			)
			if assert.NotNil(t, updateResponse.JSON400, "error response should not be nil") {
				assert.Contains(t, updateResponse.JSON400.Errors[0].Code, "VALIDATION_ERROR")
				assert.Contains(
					t, updateResponse.JSON400.Errors[0].Message, "Name cannot be blank",
				)
			}
		},
	)

	t.Run(
		"UpdateOrganizationWithInvalidNameLength", func(t *testing.T) {
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Valid Name Length Test Org",
					Description: ptr.Ptr("Valid description"),
				},
			)

			require.NoError(t, err, "create organization should not error")
			require.NotNil(t, createResponse.JSON201, "create response should not be nil")
			organizationID := createResponse.JSON201.Id

			updateResponse, err := apiKeyClient.UpdateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
				ct.UpdateProductOrganizationJSONRequestBody{
					Name:        "A",
					Description: ptr.Ptr("Valid description"),
				},
			)

			require.NoError(t, err, "update organization with short name should not error")
			assert.NotNil(t, updateResponse, "response should not be nil")
			assert.Equal(
				t, http.StatusBadRequest, updateResponse.StatusCode(),
				"update organization with short name should return 400 Bad Request",
			)
			if assert.NotNil(t, updateResponse.JSON400, "error response should not be nil") {
				assert.Contains(t, updateResponse.JSON400.Errors[0].Code, "VALIDATION_ERROR")
				assert.Contains(
					t, updateResponse.JSON400.Errors[0].Message,
					"Name must be at least 2 characters in length",
				)
			}
		},
	)

	t.Run(
		"UpdateOrganizationWithInvalidDescriptionLength", func(t *testing.T) {
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Valid Description Length Test Org",
					Description: ptr.Ptr("Valid description"),
				},
			)

			require.NoError(t, err, "create organization should not error")
			require.NotNil(t, createResponse.JSON201, "create response should not be nil")
			organizationID := createResponse.JSON201.Id

			updateResponse, err := apiKeyClient.UpdateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
				ct.UpdateProductOrganizationJSONRequestBody{
					Name:        "Valid Organization",
					Description: ptr.Ptr(generateString(501)), // Too long (maximum is 500)
				},
			)

			require.NoError(t, err, "update organization with long description should not error")
			assert.NotNil(t, updateResponse, "response should not be nil")
			assert.Equal(
				t, http.StatusBadRequest, updateResponse.StatusCode(),
				"update organization with long description should return 400 Bad Request",
			)
			if assert.NotNil(t, updateResponse.JSON400, "error response should not be nil") {
				assert.Contains(t, updateResponse.JSON400.Errors[0].Code, "VALIDATION_ERROR")
				assert.Contains(
					t, updateResponse.JSON400.Errors[0].Message,
					"Description must be a maximum of 500 characters in length",
				)
			}
		},
	)

	t.Run(
		"UpdateNonExistentOrganization", func(t *testing.T) {
			nonExistentOrgID := ids.MustNew("org")

			updateResponse, err := apiKeyClient.UpdateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				nonExistentOrgID,
				ct.UpdateProductOrganizationJSONRequestBody{
					Name:        "Updated Organization",
					Description: ptr.Ptr("This should fail"),
				},
			)

			require.NoError(t, err, "update non-existent organization should not error")
			assert.NotNil(t, updateResponse, "response should not be nil")
			assert.Equal(
				t, http.StatusNotFound, updateResponse.StatusCode(),
				"update non-existent organization should return 400 Bad Request",
			)
		},
	)

	t.Run(
		"SecurityUpdateOrganizationWithInvalidPermissions", func(t *testing.T) {
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Security Test Organization",
					Description: ptr.Ptr("Security test description"),
				},
			)

			require.NoError(t, err, "create organization should not error")
			require.NotNil(t, createResponse.JSON201, "create response should not be nil")
			organizationID := createResponse.JSON201.Id

			apiKeyClientWithBadPermissions, apiKeyIDBadPerm := testProduct.
				CreateAPIKeyClientWithScopes(
					[]string{"organization:read"},
				)

			updateResponse, err := apiKeyClientWithBadPermissions.UpdateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
				ct.UpdateProductOrganizationJSONRequestBody{
					Name:        "Should Not Update",
					Description: ptr.Ptr("This should fail"),
				},
			)

			require.NoError(t, err, "update organization request should not error")
			itshared.AssertProductAPIKeyInsufficientPermissions(
				t,
				updateResponse,
				apiKeyIDBadPerm,
				[]string{"organization:update"},
				[]string{"organization:read"},
			)
		},
	)
}

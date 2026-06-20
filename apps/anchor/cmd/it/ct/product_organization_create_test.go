package ct_test

import (
	"context"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductOrganizationCreate(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	apiKeyClient, _ := testProduct.CreateAPIKeyClientWithAllScopes()
	t.Run(
		"SuccessfulCreateOrganization", func(t *testing.T) {
			response, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Test Organization",
					Description: new("This is a test organization"),
				},
			)

			require.NoError(t, err, "create organization request should not error")
			assert.NotNil(t, response, "response should not be nil")

			if assert.NotNil(t, response.JSON201, "response body should not be nil") {
				assert.Equal(
					t, testProduct.ProductID, response.JSON201.ProductId, "product ID should match",
				)
				assert.Equal(
					t, "Test Organization", response.JSON201.Name, "organization name should match",
				)
				assert.Equal(
					t, "This is a test organization", *response.JSON201.Description,
					"description should match",
				)
				assert.NotEmpty(t, response.JSON201.Id, "organization ID should not be empty")
				assert.NotEmpty(t, response.JSON201.CreatedAt, "created at should not be empty")
				assert.NotEmpty(t, response.JSON201.UpdatedAt, "updated at should not be empty")
			}
		},
	)

	t.Run(
		"CreateOrganizationsWithDuplicateName", func(t *testing.T) {
			firstResponse, firstErr := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Duplicate Organization",
					Description: new("This is the first duplicate-name organization"),
				},
			)
			require.NoError(t, firstErr, "first duplicate-name create should not error")
			require.NotNil(t, firstResponse, "first response should not be nil")
			require.Equal(t, 201, firstResponse.StatusCode())
			require.NotNil(t, firstResponse.JSON201, "first response body should not be nil")

			secondResponse, secondErr := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Duplicate Organization",
					Description: new("This is the second duplicate-name organization"),
				},
			)
			require.NoError(t, secondErr, "second duplicate-name create should not error")
			require.NotNil(t, secondResponse, "second response should not be nil")
			require.Equal(t, 201, secondResponse.StatusCode())
			require.NotNil(t, secondResponse.JSON201, "second response body should not be nil")

			assert.Equal(t, firstResponse.JSON201.Name, secondResponse.JSON201.Name)
			assert.NotEqual(t, firstResponse.JSON201.Id, secondResponse.JSON201.Id)
		},
	)

	t.Run(
		"CreateOrganizationWithEmptyName", func(t *testing.T) {
			response, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "",
					Description: new("This organization has no name"),
				},
			)

			require.NoError(t, err, "create organization with empty name should not error")
			assert.NotNil(t, response, "response should not be nil")
			assert.Equal(
				t, 400, response.StatusCode(),
				"create organization with empty name should return 400 Bad Request",
			)
			if assert.NotNil(t, response.JSON400, "error response should not be nil") {
				assert.Contains(t, response.JSON400.Errors[0].Code, "VALIDATION_ERROR")
				assert.Contains(t, response.JSON400.Errors[0].Message, "Name is a required field")
			}
		},
	)

	t.Run(
		"CreateOrganizationWithInvalidNameLength", func(t *testing.T) {
			response, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "A",
					Description: new("This organization has a too short name"),
				},
			)

			require.NoError(t, err, "create organization with short name should not error")
			assert.NotNil(t, response, "response should not be nil")
			assert.Equal(
				t, 400, response.StatusCode(),
				"create organization with short name should return 400 Bad Request",
			)
			if assert.NotNil(t, response.JSON400, "error response should not be nil") {
				assert.Contains(t, response.JSON400.Errors[0].Code, "VALIDATION_ERROR")
				assert.Contains(
					t, response.JSON400.Errors[0].Message,
					"Name must be at least 2 characters in length",
				)
			}
		},
	)

	t.Run(
		"CreateOrganizationWithInvalidDescriptionLength", func(t *testing.T) {
			response, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Valid Organization Name",
					Description: new(generateString(501)), // Too long (maximum is 500)
				},
			)

			require.NoError(t, err, "create organization with long description should not error")
			assert.NotNil(t, response, "response should not be nil")
			assert.Equal(
				t, 400, response.StatusCode(),
				"create organization with long description should return 400 Bad Request",
			)
			if assert.NotNil(t, response.JSON400, "error response should not be nil") {
				assert.Contains(t, response.JSON400.Errors[0].Code, "VALIDATION_ERROR")
				assert.Contains(
					t, response.JSON400.Errors[0].Message,
					"Description must be a maximum of 500 characters in length",
				)
			}
		},
	)

	t.Run(
		"CreateOrganizationWithNonExistentProduct", func(t *testing.T) {
			nonExistentProductID := "prod_nonexistent123"

			response, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				nonExistentProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Organization for Non-existent Product",
					Description: new("This should fail"),
				},
			)

			require.NoError(t, err, "create organization for non-existent product should not error")
			assert.NotNil(t, response, "response should not be nil")
			assert.True(
				t, response.StatusCode() == 404 || response.StatusCode() == 400,
				"should return 404 or 400 for non-existent product",
			)
		},
	)
	// Security tests
	t.Run(
		"Security Create Organization with Invalid Permissions should return a forbidden error",
		func(t *testing.T) {
			apiKeyClientWithBadPermissions, apiKeyIDBadPerm := testProduct.
				CreateAPIKeyClientWithScopes(
					[]string{"organization:read"},
				)

			response, err := apiKeyClientWithBadPermissions.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Test Organization",
					Description: new("This is a test organization"),
				},
			)

			require.NoError(t, err, "create organization request should not error")
			itshared.AssertProductAPIKeyInsufficientPermissions(
				t,
				response,
				apiKeyIDBadPerm,
				[]string{"organization:create"},
				[]string{"organization:read"},
			)
		},
	)
}

func generateString(length int) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = 'a'
	}
	return string(result)
}

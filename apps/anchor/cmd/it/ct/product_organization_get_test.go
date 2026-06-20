package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductOrganizationGet(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	apiKeyClient, _ := testProduct.CreateAPIKeyClientWithAllScopes()

	t.Run(
		"SuccessfulGetOrganization", func(t *testing.T) {
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Gettable Organization",
					Description: new("Org to fetch"),
				},
			)
			require.NoError(t, err, "create organization should not error")
			require.NotNil(t, createResponse.JSON201, "create response should not be nil")
			organizationID := createResponse.JSON201.Id

			getResponse, err := apiKeyClient.GetProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
			)

			require.NoError(t, err, "get organization request should not error")
			assert.Equal(t, http.StatusOK, getResponse.StatusCode(), "should return 200 OK")

			if assert.NotNil(t, getResponse.JSON200, "response body should not be nil") {
				assert.Equal(t, organizationID, getResponse.JSON200.Id, "organization ID should match")
				assert.Equal(
					t, testProduct.ProductID, getResponse.JSON200.ProductId, "product ID should match",
				)
				assert.Equal(
					t, "Gettable Organization", getResponse.JSON200.Name, "name should match",
				)
				assert.Equal(t, "Org to fetch", *getResponse.JSON200.Description, "description should match")
				assert.NotEmpty(t, getResponse.JSON200.CreatedAt, "created at should not be empty")
				assert.NotEmpty(t, getResponse.JSON200.UpdatedAt, "updated at should not be empty")
			}
		},
	)

	t.Run(
		"GetNonExistentOrganization", func(t *testing.T) {
			getResponse, err := apiKeyClient.GetProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ids.MustNew("org"),
			)

			require.NoError(t, err, "get non-existent organization should not error")
			assert.Equal(
				t, http.StatusNotFound, getResponse.StatusCode(),
				"get non-existent organization should return 404 Not Found",
			)
		},
	)

	t.Run(
		"SecurityGetOrganizationWithInsufficientPermissions", func(t *testing.T) {
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Security Get Organization",
					Description: new("Security test"),
				},
			)
			require.NoError(t, err, "create organization should not error")
			require.NotNil(t, createResponse.JSON201, "create response should not be nil")
			organizationID := createResponse.JSON201.Id

			badClient, badKeyID := testProduct.CreateAPIKeyClientWithScopes(
				[]string{"organization:create"},
			)

			getResponse, err := badClient.GetProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
			)

			require.NoError(t, err, "get organization request should not error")
			itshared.AssertProductAPIKeyInsufficientPermissions(
				t,
				getResponse,
				badKeyID,
				[]string{"organization:read"},
				[]string{"organization:create"},
			)
		},
	)
}

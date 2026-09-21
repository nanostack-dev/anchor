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

func TestProductOrganizationDelete(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	apiKeyClient, _ := testProduct.CreateAPIKeyClientWithAllScopes()

	t.Run(
		"SuccessfulDeleteOrganization", func(t *testing.T) {
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Deletable Organization",
					Description: new("Org to delete"),
				},
			)
			require.NoError(t, err, "create organization should not error")
			require.NotNil(t, createResponse.JSON201, "create response should not be nil")
			organizationID := createResponse.JSON201.Id

			deleteResponse, err := apiKeyClient.DeleteProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
			)
			require.NoError(t, err, "delete organization request should not error")
			assert.Equal(
				t, http.StatusNoContent, deleteResponse.StatusCode(),
				"delete organization should return 204 No Content",
			)

			// Verify it is gone.
			getResponse, err := apiKeyClient.GetProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
				nil,
			)
			require.NoError(t, err, "get deleted organization should not error")
			assert.Equal(
				t, http.StatusNotFound, getResponse.StatusCode(),
				"deleted organization should no longer be retrievable",
			)
		},
	)

	t.Run(
		"DeleteNonExistentOrganization", func(t *testing.T) {
			deleteResponse, err := apiKeyClient.DeleteProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ids.MustNew("org"),
			)
			require.NoError(t, err, "delete non-existent organization should not error")
			assert.Equal(
				t, http.StatusNotFound, deleteResponse.StatusCode(),
				"delete non-existent organization should return 404 Not Found",
			)
		},
	)

	t.Run(
		"SecurityDeleteOrganizationWithInsufficientPermissions", func(t *testing.T) {
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:        "Security Delete Organization",
					Description: new("Security test"),
				},
			)
			require.NoError(t, err, "create organization should not error")
			require.NotNil(t, createResponse.JSON201, "create response should not be nil")
			organizationID := createResponse.JSON201.Id

			badClient, badKeyID := testProduct.CreateAPIKeyClientWithScopes(
				[]string{"organization:read"},
			)

			deleteResponse, err := badClient.DeleteProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				organizationID,
			)
			require.NoError(t, err, "delete organization request should not error")
			itshared.AssertProductAPIKeyInsufficientPermissions(
				t,
				deleteResponse,
				badKeyID,
				[]string{"organization:delete"},
				[]string{"organization:read"},
			)
		},
	)

	t.Run("EmitsWebhook", func(t *testing.T) {
		product := createTestProductContext(t)
		sink := product.CaptureEvents()
		client, _ := product.CreateAPIKeyClientWithAllScopes()
		created, err := client.CreateProductOrganizationWithResponse(
			ctx,
			product.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{Name: "Webhook Delete Org"},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode())
		deleted, deleteErr := client.DeleteProductOrganizationWithResponse(
			ctx, product.ProductID, created.JSON201.Id,
		)
		require.NoError(t, deleteErr)
		require.Equal(t, http.StatusNoContent, deleted.StatusCode())
		sink.WaitFor("organization.deleted", map[string]string{"organization_id": created.JSON201.Id})
	})
}

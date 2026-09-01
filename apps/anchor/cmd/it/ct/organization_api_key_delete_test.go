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

func TestOrganizationAPIKeyDelete(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	apiKeyClient, _ := product.CreateAPIKeyClientWithScopes([]string{
		"organization_api_key:create",
		"organization_api_key:delete",
		"organization_api_key:read",
	})
	permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
	org := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)

	createResp, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
		ctx,
		product.ProductID,
		org.Id,
		ct.CreateOrganizationAPIKeyJSONRequestBody{
			Name:        "DeleteKey-" + uuid.NewString(),
			Permissions: []string{permissions.FileRead},
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	require.NotNil(t, createResp.JSON201)

	t.Run("Delete existing organization API key successfully", func(t *testing.T) {
		deleteResp, deleteErr := apiKeyClient.DeleteOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			createResp.JSON201.Id,
		)
		require.NoError(t, deleteErr)
		assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode())

		getResp, getErr := apiKeyClient.GetOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			createResp.JSON201.Id,
		)
		require.NoError(t, getErr)
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode())
	})

	t.Run("Delete non-existent organization API key returns not found", func(t *testing.T) {
		deleteResp, deleteErr := apiKeyClient.DeleteOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			ids.MustNew("organization_apikey"),
		)
		require.NoError(t, deleteErr)
		assert.Equal(t, http.StatusNotFound, deleteResp.StatusCode())
	})

	t.Run("EmitsWebhook", func(t *testing.T) {
		webhookProduct := createTestProductContext(t)
		sink := webhookProduct.CaptureEvents()
		webhookClient, _ := webhookProduct.CreateAPIKeyClientWithAllScopes()
		webhookPermissions := givenOrganizationAPIKeyResourcePermissions(t, webhookProduct)
		webhookOrg := webhookProduct.CreateOrganization(t, "Webhook-APIKey-Delete-"+uuid.NewString(), nil)
		created, createErr := webhookClient.CreateOrganizationAPIKeyWithResponse(
			ctx,
			webhookProduct.ProductID,
			webhookOrg.Id,
			ct.CreateOrganizationAPIKeyJSONRequestBody{
				Name:        "WebhookDeleteKey-" + uuid.NewString(),
				Permissions: []string{webhookPermissions.FileRead},
			},
		)
		require.NoError(t, createErr)
		require.Equal(t, http.StatusCreated, created.StatusCode())
		deleted, deleteErr := webhookClient.DeleteOrganizationAPIKeyWithResponse(
			ctx, webhookProduct.ProductID, webhookOrg.Id, created.JSON201.Id,
		)
		require.NoError(t, deleteErr)
		require.Equal(t, http.StatusNoContent, deleted.StatusCode())
		sink.WaitFor("organization.api_key.deleted", map[string]string{
			"organization_id": webhookOrg.Id,
			"api_key_id":      created.JSON201.Id,
		})
	})
}

package ct_test

import (
	"context"
	"net/http"
	"testing"

	itshared "anchor/cmd/it/shared"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationAPIKeyUpdate(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	apiKeyClient, _ := product.CreateAPIKeyClientWithScopes([]string{
		"organization_api_key:create",
		"organization_api_key:update",
	})
	description := itshared.Faker.Lorem().Sentence(4)
	org := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description)
	permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
	originalDescription := "Original description"

	createResp, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
		ctx,
		product.ProductID,
		org.Id,
		ct.CreateOrganizationAPIKeyJSONRequestBody{
			Name:        "UpdateKey-" + uuid.NewString(),
			Description: &originalDescription,
			Permissions: []string{permissions.FileRead},
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())

	t.Run("Update organization API key name and description", func(t *testing.T) {
		newName := "Updated-" + uuid.NewString()
		newDescription := "Updated description"
		updateResp, updateErr := apiKeyClient.UpdateOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			createResp.JSON201.Id,
			ct.UpdateOrganizationAPIKeyJSONRequestBody{
				Name:        newName,
				Description: &newDescription,
			},
		)
		require.NoError(t, updateErr)
		require.Equal(t, http.StatusOK, updateResp.StatusCode())
		require.NotNil(t, updateResp.JSON200)
		assert.Equal(t, newName, updateResp.JSON200.Name)
		assert.Equal(t, newDescription, *updateResp.JSON200.Description)
		assert.Equal(t, createResp.JSON201.ObfuscatedValue, updateResp.JSON200.ObfuscatedValue)
		require.Len(t, updateResp.JSON200.Permissions, 1)
		assert.Equal(t, permissions.FileRead, updateResp.JSON200.Permissions[0].PermissionName)
	})

	t.Run("Update organization API key with blank name returns bad request", func(t *testing.T) {
		updateResp, updateErr := apiKeyClient.UpdateOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			createResp.JSON201.Id,
			ct.UpdateOrganizationAPIKeyJSONRequestBody{Name: "   "},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusBadRequest, updateResp.StatusCode())
		require.NotNil(t, updateResp.JSON400)
		assert.Contains(t, updateResp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
	})

	t.Run("Update non-existent organization API key returns not found", func(t *testing.T) {
		newName := "Missing-" + uuid.NewString()
		updateResp, updateErr := apiKeyClient.UpdateOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			ids.MustNew("organization_apikey"),
			ct.UpdateOrganizationAPIKeyJSONRequestBody{Name: newName},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusNotFound, updateResp.StatusCode())
	})

	t.Run("EmitsWebhook", func(t *testing.T) {
		webhookProduct := createTestProductContext(t)
		sink := webhookProduct.CaptureEvents()
		webhookClient, _ := webhookProduct.CreateAPIKeyClientWithAllScopes()
		webhookPermissions := givenOrganizationAPIKeyResourcePermissions(t, webhookProduct)
		webhookOrg := webhookProduct.CreateOrganization(t, "Webhook-APIKey-Update-"+uuid.NewString(), nil)
		created, err := webhookClient.CreateOrganizationAPIKeyWithResponse(
			ctx,
			webhookProduct.ProductID,
			webhookOrg.Id,
			ct.CreateOrganizationAPIKeyJSONRequestBody{
				Name:        "WebhookUpdateKey-" + uuid.NewString(),
				Permissions: []string{webhookPermissions.FileRead},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode())
		updated, updateErr := webhookClient.UpdateOrganizationAPIKeyWithResponse(
			ctx,
			webhookProduct.ProductID,
			webhookOrg.Id,
			created.JSON201.Id,
			ct.UpdateOrganizationAPIKeyJSONRequestBody{Name: "WebhookUpdated-" + uuid.NewString()},
		)
		require.NoError(t, updateErr)
		require.Equal(t, http.StatusOK, updated.StatusCode())
		sink.WaitFor("organization.api_key.updated", map[string]string{
			"organization_id": webhookOrg.Id,
			"api_key_id":      created.JSON201.Id,
		})
	})
}

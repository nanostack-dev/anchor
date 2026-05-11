package ct_test

import (
	"context"
	"net/http"
	"testing"

	itshared "anchor/cmd/it/shared"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/shared/toolkit"

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
				Name:        &newName,
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

	t.Run("Update non-existent organization API key returns not found", func(t *testing.T) {
		newName := "Missing-" + uuid.NewString()
		updateResp, updateErr := apiKeyClient.UpdateOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			toolkit.NewID("organization_apikey"),
			ct.UpdateOrganizationAPIKeyJSONRequestBody{Name: &newName},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusNotFound, updateResp.StatusCode())
	})
}

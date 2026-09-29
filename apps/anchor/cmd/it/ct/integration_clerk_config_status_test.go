package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestClerkIntegrationConfigAndStatus(t *testing.T) {
	t.Run("RejectsWebhookWhenSecretIsNotConfigured", func(t *testing.T) {
		productContext := createTestProductContext(t)
		createClerkIntegrationInstance(t, productContext)

		externalID := "user_clerk_" + itshared.Faker.UUID().V4()
		payload := clerkUserCreatedPayload(t, externalID, itshared.Faker.Internet().Email(), "John", "Doe")

		resp := sendClerkWebhook(t, productContext.ProductID, payload, clerkTestWebhookSecret)
		assert.Equal(t, http.StatusConflict, resp.StatusCode())
		require.NotNil(t, resp.JSON409)
		require.Len(t, resp.JSON409.Errors, 1)
		assert.Equal(t, "INTEGRATION_INSTANCE_CONFIGURING", resp.JSON409.Errors[0].Code)
	})

	t.Run("ConfiguredSecretAndActiveStatusProcessWebhook", func(t *testing.T) {
		productContext := createTestProductContext(t)
		instance := createActiveClerkIntegrationInstance(t, productContext)

		externalID := "user_clerk_" + itshared.Faker.UUID().V4()
		email := itshared.Faker.Internet().Email()
		payload := clerkUserCreatedPayload(t, externalID, email, "Config", "Ready")

		webhookResp := sendClerkWebhook(t, productContext.ProductID, payload, clerkTestWebhookSecret)
		assert.Equal(t, http.StatusOK, webhookResp.StatusCode())
		require.NotNil(t, webhookResp.JSON200)
		assert.Equal(t, ct.IntegrationEventStatusPENDING, webhookResp.JSON200.Status)

		getResp := getIntegrationInstance(t, productContext, instance.Id)
		assert.Equal(t, ct.IntegrationInstanceStatusACTIVE, getResp.Status)

		listResp := listIntegrationInstances(t, productContext)
		require.Len(t, listResp.Items, 1)
		assert.Equal(t, ct.IntegrationInstanceStatusACTIVE, listResp.Items[0].Status)

		require.Eventually(t, func() bool {
			searchResp := searchProductUsers(t, productContext)
			return searchResp.JSON200.Total == 1
		}, 5*time.Second, 200*time.Millisecond)
	})

	t.Run("InactiveInstanceRejectsWebhook", func(t *testing.T) {
		productContext := createTestProductContext(t)
		instance := createActiveClerkIntegrationInstance(t, productContext)

		updated := updateClerkIntegrationInstance(
			t,
			productContext,
			instance.Id,
			ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(false)},
		)
		assert.Equal(t, ct.IntegrationInstanceStatusINACTIVE, updated.Status)

		externalID := "user_clerk_" + itshared.Faker.UUID().V4()
		payload := clerkUserCreatedPayload(t, externalID, itshared.Faker.Internet().Email(), "John", "Doe")

		resp := sendClerkWebhook(t, productContext.ProductID, payload, clerkTestWebhookSecret)
		assert.Equal(t, http.StatusConflict, resp.StatusCode())
		require.NotNil(t, resp.JSON409)
		require.Len(t, resp.JSON409.Errors, 1)
		assert.Equal(t, "INTEGRATION_INSTANCE_DISABLED", resp.JSON409.Errors[0].Code)
	})

	t.Run("EnableFlagPromotesInactiveToActive", func(t *testing.T) {
		productContext := createTestProductContext(t)
		instance := createActiveClerkIntegrationInstance(t, productContext)

		updatedToInactive := updateClerkIntegrationInstance(
			t,
			productContext,
			instance.Id,
			ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(false)},
		)
		assert.Equal(t, ct.IntegrationInstanceStatusINACTIVE, updatedToInactive.Status)

		updatedToActive := updateClerkIntegrationInstance(
			t,
			productContext,
			instance.Id,
			ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(true)},
		)
		assert.Equal(t, ct.IntegrationInstanceStatusACTIVE, updatedToActive.Status)
		assert.True(t, updatedToActive.IsEnabled)

		fetched := getIntegrationInstance(t, productContext, instance.Id)
		assert.Equal(t, ct.IntegrationInstanceStatusACTIVE, fetched.Status)
		assert.True(t, fetched.IsEnabled)
	})

	t.Run("EnableFlagDoesNotPromoteConfiguringToActive", func(t *testing.T) {
		productContext := createTestProductContext(t)
		instance := createClerkIntegrationInstance(t, productContext)

		trueValue := true
		resp, err := productContext.OwnerAuthenticatedClient().UpdateIntegrationInstanceWithResponse(
			context.Background(),
			productContext.ProductID,
			instance.Id,
			ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: &trueValue},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, ct.IntegrationInstanceStatusCONFIGURING, resp.JSON200.Status)
		assert.True(t, resp.JSON200.IsEnabled)

		fetched := getIntegrationInstance(t, productContext, instance.Id)
		assert.Equal(t, ct.IntegrationInstanceStatusCONFIGURING, fetched.Status)
		assert.True(t, fetched.IsEnabled)
	})

	t.Run("NoopUpdateDoesNotPromoteConfiguringWithoutSecret", func(t *testing.T) {
		productContext := createTestProductContext(t)
		instance := createClerkIntegrationInstance(t, productContext)

		resp, err := productContext.OwnerAuthenticatedClient().UpdateIntegrationInstanceWithResponse(
			context.Background(),
			productContext.ProductID,
			instance.Id,
			ct.UpdateIntegrationInstanceJSONRequestBody{},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, ct.IntegrationInstanceStatusCONFIGURING, resp.JSON200.Status)
	})

	t.Run("LifecycleTransitionsArePersisted", func(t *testing.T) {
		productContext := createTestProductContext(t)
		instance := createClerkIntegrationInstance(t, productContext)

		updatedToActive := updateClerkIntegrationInstance(
			t,
			productContext,
			instance.Id,
			ct.UpdateIntegrationInstanceJSONRequestBody{WebhookSecret: new(clerkTestWebhookSecret)},
		)
		assert.Equal(t, ct.IntegrationInstanceStatusACTIVE, updatedToActive.Status)

		updatedToInactive := updateClerkIntegrationInstance(
			t,
			productContext,
			instance.Id,
			ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(false)},
		)
		assert.Equal(t, ct.IntegrationInstanceStatusINACTIVE, updatedToInactive.Status)

		updatedToActiveAgain := updateClerkIntegrationInstance(
			t,
			productContext,
			instance.Id,
			ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(true)},
		)
		assert.Equal(t, ct.IntegrationInstanceStatusACTIVE, updatedToActiveAgain.Status)

		listResp := listIntegrationInstances(t, productContext)
		require.Len(t, listResp.Items, 1)
		assert.Equal(t, ct.IntegrationInstanceStatusACTIVE, listResp.Items[0].Status)
	})
}

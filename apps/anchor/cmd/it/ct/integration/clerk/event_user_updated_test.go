package ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestClerkWebhookUserUpdatedEvent(t *testing.T) {
	productContext := createTestProductContext(t)
	createActiveClerkIntegrationInstance(t, productContext)

	externalID := "user_clerk_" + itshared.Faker.UUID().V4()
	initialEmail := itshared.Faker.Internet().Email()

	createdPayload := clerkUserCreatedPayload(t, externalID, initialEmail, "Original", "User")
	createdResp := sendClerkWebhook(
		t, productContext.ProductID, createdPayload, clerkTestWebhookSecret,
	)
	require.Equal(t, http.StatusOK, createdResp.StatusCode())
	require.Eventually(
		t, func() bool {
			searchResp := searchProductUsers(t, productContext)
			if searchResp.JSON200.Total != 1 || len(searchResp.JSON200.Items) != 1 {
				return false
			}
			return searchResp.JSON200.Items[0].Email == initialEmail
		}, 10*time.Second, 200*time.Millisecond,
	)

	updatedEmail := itshared.Faker.Internet().Email()
	updatedPayload := clerkUserUpdatedPayload(t, externalID, updatedEmail, "Updated", "Name")
	updatedResp := sendClerkWebhook(
		t, productContext.ProductID, updatedPayload, clerkTestWebhookSecret,
	)
	assert.Equal(t, http.StatusOK, updatedResp.StatusCode())
	require.NotNil(t, updatedResp.JSON200)
	assert.Equal(t, ct.IntegrationEventStatusPENDING, updatedResp.JSON200.Status)

	require.Eventually(
		t, func() bool {
			searchResp := searchProductUsers(t, productContext)
			if searchResp.JSON200.Total != 1 || len(searchResp.JSON200.Items) != 1 {
				return false
			}

			user := searchResp.JSON200.Items[0]
			return user.Email == updatedEmail && user.Name != nil && *user.Name == "Updated Name"
		}, 10*time.Second, 200*time.Millisecond,
	)
}

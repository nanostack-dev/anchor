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

func TestClerkWebhookUserCreatedEvent(t *testing.T) {
	productContext := createTestProductContext(t)
	createActiveClerkIntegrationInstance(t, productContext)

	externalID := "user_clerk_" + itshared.Faker.UUID().V4()
	email := itshared.Faker.Internet().Email()
	firstName := itshared.Faker.Person().FirstName()
	lastName := itshared.Faker.Person().LastName()
	payload := clerkUserCreatedPayload(t, externalID, email, firstName, lastName)

	webhookResp := sendClerkWebhook(t, productContext.ProductID, payload, clerkTestWebhookSecret)
	assert.Equal(t, http.StatusOK, webhookResp.StatusCode())
	require.NotNil(t, webhookResp.JSON200)

	// Webhook now returns PENDING; processing happens asynchronously.
	assert.Equal(t, ct.IntegrationEventStatusPENDING, webhookResp.JSON200.Status)

	require.Eventually(t, func() bool {
		searchResp := searchProductUsers(t, productContext)
		if searchResp.JSON200.Total != 1 || len(searchResp.JSON200.Items) != 1 {
			return false
		}

		user := searchResp.JSON200.Items[0]
		return user.Email == email &&
			user.Name != nil && *user.Name == firstName+" "+lastName
	}, 10*time.Second, 200*time.Millisecond)
}

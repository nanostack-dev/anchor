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

func TestClerkWebhookUserDeletedEvent(t *testing.T) {
	productContext := createTestProductContext(t)
	createActiveClerkIntegrationInstance(t, productContext)

	externalID := "user_clerk_" + itshared.Faker.UUID().V4()
	createdPayload := clerkUserCreatedPayload(t, externalID, itshared.Faker.Internet().Email(), "Delete", "Me")
	createdResp := sendClerkWebhook(t, productContext.ProductID, createdPayload, clerkTestWebhookSecret)
	require.Equal(t, http.StatusOK, createdResp.StatusCode())
	require.Eventually(t, func() bool {
		beforeDelete := searchProductUsers(t, productContext)
		return beforeDelete.JSON200.Total == 1
	}, 10*time.Second, 200*time.Millisecond)

	deletedPayload := clerkUserDeletedPayload(t, externalID)
	deletedResp := sendClerkWebhook(t, productContext.ProductID, deletedPayload, clerkTestWebhookSecret)
	assert.Equal(t, http.StatusOK, deletedResp.StatusCode())
	require.NotNil(t, deletedResp.JSON200)
	assert.Equal(t, ct.IntegrationEventStatusPENDING, deletedResp.JSON200.Status)

	require.Eventually(t, func() bool {
		afterDelete := searchProductUsers(t, productContext)
		return afterDelete.JSON200.Total == 0
	}, 10*time.Second, 200*time.Millisecond)
}

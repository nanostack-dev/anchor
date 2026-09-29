package ct_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestClerkWebhookBackToBackCreatedThenUpdatedEmitsOneOfEach(t *testing.T) {
	t.Parallel()
	productContext := createTestProductContext(t)
	sink := productContext.CaptureEvents()
	createActiveClerkIntegrationInstance(t, productContext)

	externalID := "user_clerk_" + itshared.Faker.UUID().V4()
	createdPayload := clerkUserCreatedPayload(
		t, externalID, itshared.Faker.Internet().Email(), "Original", "User",
	)
	updatedPayload := clerkUserUpdatedPayload(
		t, externalID, itshared.Faker.Internet().Email(), "Updated", "Name",
	)

	createdResp := sendClerkWebhook(t, productContext.ProductID, createdPayload, clerkTestWebhookSecret)
	require.Equal(t, http.StatusOK, createdResp.StatusCode())
	updatedResp := sendClerkWebhook(t, productContext.ProductID, updatedPayload, clerkTestWebhookSecret)
	require.Equal(t, http.StatusOK, updatedResp.StatusCode())

	created := sink.WaitFor("product_user.created", nil)
	productUserID := created.Field("product_user_id")
	require.NotEmpty(t, productUserID)
	sink.WaitFor("product_user.updated", map[string]string{"product_user_id": productUserID})

	require.Equal(t, 1, sink.Count("product_user.created"))
	require.Equal(t, 1, sink.Count("product_user.updated"))
}

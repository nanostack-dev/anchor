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
	sink := productContext.CaptureEvents()
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

	updatedUser := searchProductUsers(t, productContext).JSON200.Items[0]
	sink.WaitFor("product_user.updated", map[string]string{"product_user_id": updatedUser.Id})
}

func TestClerkWebhookUnchangedUserDoesNotEmit(t *testing.T) {
	productContext := createTestProductContext(t)
	sink := productContext.CaptureEvents()
	createActiveClerkIntegrationInstance(t, productContext)

	externalID := "user_clerk_" + itshared.Faker.UUID().V4()
	email := itshared.Faker.Internet().Email()
	createdPayload := clerkUserCreatedPayload(t, externalID, email, "Stable", "User")
	createdResp := sendClerkWebhook(
		t, productContext.ProductID, createdPayload, clerkTestWebhookSecret,
	)
	require.Equal(t, http.StatusOK, createdResp.StatusCode())

	require.Eventually(
		t, func() bool {
			searchResp := searchProductUsers(t, productContext)
			return searchResp.JSON200.Total == 1 && len(searchResp.JSON200.Items) == 1
		}, 10*time.Second, 200*time.Millisecond,
	)

	createdUser := searchProductUsers(t, productContext).JSON200.Items[0]
	sink.WaitFor("product_user.created", map[string]string{"product_user_id": createdUser.Id})
	require.Equal(t, 1, sink.Count("product_user.created"))
	require.Equal(t, 0, sink.Count("product_user.updated"))

	unchangedPayload := clerkUserUpdatedPayload(t, externalID, email, "Stable", "User")
	unchangedResp := sendClerkWebhook(
		t, productContext.ProductID, unchangedPayload, clerkTestWebhookSecret,
	)
	require.Equal(t, http.StatusOK, unchangedResp.StatusCode())
	require.NotNil(t, unchangedResp.JSON200)
	assert.Equal(t, ct.IntegrationEventStatusPENDING, unchangedResp.JSON200.Status)

	require.Never(
		t, func() bool {
			return sink.Count("product_user.updated") > 0
		}, 6*time.Second, 200*time.Millisecond,
	)
	assert.Equal(t, 1, sink.Count("product_user.created"))
}

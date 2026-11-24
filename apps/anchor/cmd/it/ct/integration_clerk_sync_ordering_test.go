package ct_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
)

func setClerkEventTime(t *testing.T, payload map[string]any, tsMs int64) {
	t.Helper()
	payload["timestamp"] = float64(tsMs)
	data := payloadDataMap(t, payload)
	if _, hasUpdatedAt := data["updated_at"]; hasUpdatedAt {
		data["updated_at"] = float64(tsMs)
	}
}

func waitForProcessedWebhooks(
	t *testing.T,
	productContext *itdsl.ProductContext,
	instanceID string,
	wantCount int,
) {
	t.Helper()
	require.Eventually(t, func() bool {
		logs := listIntegrationAuditLogs(t, productContext, instanceID)
		processed := 0
		for _, item := range logs.Items {
			if item.Action == "WEBHOOK_PROCESSED" {
				processed++
			}
		}
		return processed >= wantCount
	}, 10*time.Second, 200*time.Millisecond)
}

func TestClerkWebhookEventOrderingAndTombstones(t *testing.T) {
	t.Parallel()

	t.Run("Stale user.updated arriving after user.deleted does not resurrect the user", func(t *testing.T) {
		t.Parallel()

		productContext := createTestProductContext(t)
		inst := createActiveClerkIntegrationInstance(t, productContext)

		externalID := "user_clerk_" + itshared.Faker.UUID().V4()
		email := itshared.Faker.Internet().Email()
		baseMs := int64(1730000000000)

		created := clerkUserCreatedPayload(t, externalID, email, "Alice", "Initial")
		setClerkEventTime(t, created, baseMs)
		require.Equal(
			t,
			http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, created, clerkTestWebhookSecret).StatusCode(),
		)
		require.Eventually(t, func() bool {
			return searchProductUsers(t, productContext).JSON200.Total == 1
		}, 10*time.Second, 200*time.Millisecond)

		deleted := clerkUserDeletedPayload(t, externalID)
		setClerkEventTime(t, deleted, baseMs+60_000)
		require.Equal(
			t,
			http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, deleted, clerkTestWebhookSecret).StatusCode(),
		)
		require.Eventually(t, func() bool {
			return searchProductUsers(t, productContext).JSON200.Total == 0
		}, 10*time.Second, 200*time.Millisecond)

		// Send a late update whose updated_at predates the delete.
		staleUpdate := clerkUserUpdatedPayload(t, externalID, email, "Alice", "Zombie")
		setClerkEventTime(t, staleUpdate, baseMs+30_000)
		require.Equal(
			t,
			http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, staleUpdate, clerkTestWebhookSecret).StatusCode(),
		)

		waitForProcessedWebhooks(t, productContext, inst.Id, 3)

		assert.Equal(t, int64(0), searchProductUsers(t, productContext).JSON200.Total,
			"stale update after delete must not resurrect the deleted user")
	})

	t.Run("Delete arriving before user.created records a tombstone and blocks the late create", func(t *testing.T) {
		t.Parallel()

		productContext := createTestProductContext(t)
		inst := createActiveClerkIntegrationInstance(t, productContext)

		externalID := "user_clerk_" + itshared.Faker.UUID().V4()
		email := itshared.Faker.Internet().Email()
		baseMs := int64(1730000100000)

		deleted := clerkUserDeletedPayload(t, externalID)
		setClerkEventTime(t, deleted, baseMs+60_000)
		require.Equal(
			t,
			http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, deleted, clerkTestWebhookSecret).StatusCode(),
		)
		waitForProcessedWebhooks(t, productContext, inst.Id, 1)

		lateCreate := clerkUserCreatedPayload(t, externalID, email, "Ghost", "User")
		setClerkEventTime(t, lateCreate, baseMs)
		require.Equal(
			t,
			http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, lateCreate, clerkTestWebhookSecret).StatusCode(),
		)
		waitForProcessedWebhooks(t, productContext, inst.Id, 2)

		assert.Equal(t, int64(0), searchProductUsers(t, productContext).JSON200.Total,
			"tombstone must prevent late user.created from creating the deleted user")
	})

	t.Run("Older user.updated does not overwrite newer name or email", func(t *testing.T) {
		t.Parallel()

		productContext := createTestProductContext(t)
		inst := createActiveClerkIntegrationInstance(t, productContext)

		externalID := "user_clerk_" + itshared.Faker.UUID().V4()
		newerEmail := "new-" + itshared.Faker.Internet().Email()
		olderEmail := "old-" + itshared.Faker.Internet().Email()
		baseMs := int64(1730000200000)

		newerUpdate := clerkUserUpdatedPayload(t, externalID, newerEmail, "Newer", "Name")
		setClerkEventTime(t, newerUpdate, baseMs+60_000)
		require.Equal(
			t,
			http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, newerUpdate, clerkTestWebhookSecret).StatusCode(),
		)
		require.Eventually(t, func() bool {
			res := searchProductUsers(t, productContext)
			return res.JSON200.Total == 1 && res.JSON200.Items[0].Email == newerEmail
		}, 10*time.Second, 200*time.Millisecond)

		olderUpdate := clerkUserUpdatedPayload(t, externalID, olderEmail, "Older", "Name")
		setClerkEventTime(t, olderUpdate, baseMs)
		require.Equal(
			t,
			http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, olderUpdate, clerkTestWebhookSecret).StatusCode(),
		)
		waitForProcessedWebhooks(t, productContext, inst.Id, 2)

		res := searchProductUsers(t, productContext)
		require.Equal(t, int64(1), res.JSON200.Total)
		assert.Equal(t, newerEmail, res.JSON200.Items[0].Email)
		require.NotNil(t, res.JSON200.Items[0].Name)
		assert.Equal(t, "Newer Name", *res.JSON200.Items[0].Name)
	})

	t.Run("Deleting a user and signing up again with the same email creates the new external ID", func(t *testing.T) {
		t.Parallel()

		productContext := createTestProductContext(t)
		createActiveClerkIntegrationInstance(t, productContext)

		email := "resignup-" + itshared.Faker.Internet().Email()
		oldExternalID := "user_clerk_old_" + itshared.Faker.UUID().V4()
		newExternalID := "user_clerk_new_" + itshared.Faker.UUID().V4()
		baseMs := int64(1730000300000)

		createdOld := clerkUserCreatedPayload(t, oldExternalID, email, "Old", "Account")
		setClerkEventTime(t, createdOld, baseMs)
		require.Equal(
			t,
			http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, createdOld, clerkTestWebhookSecret).StatusCode(),
		)
		require.Eventually(t, func() bool {
			return searchProductUsers(t, productContext).JSON200.Total == 1
		}, 10*time.Second, 200*time.Millisecond)

		deletedOld := clerkUserDeletedPayload(t, oldExternalID)
		setClerkEventTime(t, deletedOld, baseMs+60_000)
		require.Equal(
			t,
			http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, deletedOld, clerkTestWebhookSecret).StatusCode(),
		)
		require.Eventually(t, func() bool {
			return searchProductUsers(t, productContext).JSON200.Total == 0
		}, 10*time.Second, 200*time.Millisecond)

		createdNew := clerkUserCreatedPayload(t, newExternalID, email, "New", "Account")
		setClerkEventTime(t, createdNew, baseMs+120_000)
		require.Equal(
			t,
			http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, createdNew, clerkTestWebhookSecret).StatusCode(),
		)
		require.Eventually(t, func() bool {
			res := searchProductUsers(t, productContext)
			return res.JSON200.Total == 1 &&
				res.JSON200.Items[0].ExternalId != nil &&
				*res.JSON200.Items[0].ExternalId == newExternalID
		}, 10*time.Second, 200*time.Millisecond)
	})
}

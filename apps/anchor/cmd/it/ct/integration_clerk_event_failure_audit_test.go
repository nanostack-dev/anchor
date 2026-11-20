package ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

// Regression for docs/postmortems/2026-11-19-failure-audit-in-aborted-tx.md.
func TestClerkWebhookFailedCommandAudit(t *testing.T) {
	t.Parallel()

	t.Run("A Clerk user event whose upsert fails leaves an ERROR upsert audit row", func(t *testing.T) {
		t.Parallel()

		productContext := createTestProductContext(t)
		instance := createActiveClerkIntegrationInstance(t, productContext)

		email := itshared.Faker.Internet().Email()
		first := clerkUserCreatedPayload(
			t, "user_clerk_"+itshared.Faker.UUID().V4(), email, "First", "Holder",
		)
		require.Equal(t, http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, first, clerkTestWebhookSecret).StatusCode())
		require.Eventually(t, func() bool {
			return searchProductUsers(t, productContext).JSON200.Total == 1
		}, 10*time.Second, 200*time.Millisecond)

		// A second Clerk user with the same email violates
		// product_users_product_id_email_key, which aborts the command transaction.
		secondExternalID := "user_clerk_" + itshared.Faker.UUID().V4()
		second := clerkUserCreatedPayload(t, secondExternalID, email, "Second", "Holder")
		require.Equal(t, http.StatusOK,
			sendClerkWebhook(t, productContext.ProductID, second, clerkTestWebhookSecret).StatusCode())

		require.Eventually(t, func() bool {
			for _, item := range listIntegrationAuditLogs(t, productContext, instance.Id).Items {
				if item.Action == "UPSERT_USER" &&
					item.Severity == ct.IntegrationAuditLogSeverityERROR &&
					item.Metadata != nil && (*item.Metadata)["external_id"] == secondExternalID {
					return true
				}
			}
			return false
		}, 15*time.Second, 250*time.Millisecond, "the failed upsert left no ERROR audit row")
	})
}

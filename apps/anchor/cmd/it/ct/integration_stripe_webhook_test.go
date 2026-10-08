package ct_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v87/webhook"

	itshared "anchor/cmd/it/shared"
	dslfactory "anchor/cmd/it/shared/dsl/factory"
	"anchor/internal/domain/integration"
	stripeprovider "anchor/internal/integration/provider/stripe"
	"anchor/internal/stripebilling"
	"anchor/internal/stripebilling/billing"
)

func seedActiveStripeWebhookInstance(t *testing.T, tc webhookSecretTestCtx) integration.Instance {
	t.Helper()
	body, err := json.Marshal(stripeprovider.Config{
		AuthMethod: stripeprovider.AuthMethodAPIKey, AccountID: stripeFixtureAccount,
		APIKey: stripeFixtureKey, WebhookSecret: stripeFixtureWebhook,
		ReturnURL: "https://anchor.example.test",
	})
	require.NoError(t, err)
	stored, err := StripeProvider.PrepareConfigForStorage(t.Context(), body)
	require.NoError(t, err)
	instance := integration.Instance{
		PlatformTenantID: tc.tenantID, ProductID: tc.product.ProductID, ProviderType: integration.ProviderTypeStripe,
		ConfigJSON: stored, ConfigVersion: 1, IsEnabled: true, Status: integration.StatusActive,
	}
	instance.GenerateID()
	created, err := IntegrationRepo.Create(t.Context(), instance)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, IntegrationRepo.DeleteByID(context.Background(), tc.tenantID, created.ID))
	})
	return created
}

func sendStripeContractEvent(t *testing.T, productID, body, signature string) *ct.IngestStripeBillingWebhookResponse {
	t.Helper()
	response, err := dslfactory.NewNoAuthClient(t, itshared.ServerURL).
		IngestStripeBillingWebhookWithBodyWithResponse(t.Context(), productID,
			&ct.IngestStripeBillingWebhookParams{
				StripeSignature: signature,
			}, "application/json", strings.NewReader(body))
	require.NoError(t, err)
	return response
}

func signedStripeContractEvent(body string) string {
	return webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: []byte(body), Secret: stripeFixtureWebhook,
	}).Header
}

func storedStripeEvents(t *testing.T, instance integration.Instance) map[string]json.RawMessage {
	t.Helper()
	var body []byte
	require.NoError(t, testDB.QueryRowContext(t.Context(), `SELECT state_json FROM stripe_billing_states
		WHERE integration_instance_id=$1 AND platform_tenant_id=$2 AND product_id=$3`,
		instance.ID, instance.PlatformTenantID, instance.ProductID).Scan(&body))
	var state struct {
		Events map[string]json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(body, &state))
	return state.Events
}

func requireNoStripeBillingState(t *testing.T, instance integration.Instance) {
	t.Helper()
	var count int
	require.NoError(t, testDB.QueryRowContext(t.Context(), `SELECT count(*) FROM stripe_billing_states
		WHERE integration_instance_id=$1 AND platform_tenant_id=$2 AND product_id=$3`,
		instance.ID, instance.PlatformTenantID, instance.ProductID).Scan(&count))
	require.Zero(t, count, "refused webhook must not initialize billing state")
}

func TestStripeWebhookUsesRawSignedBodyAndRejectsLiveOrForeignAccount(t *testing.T) {
	t.Parallel()
	tc := newWebhookSecretTestCtx(t)
	instance := seedActiveStripeWebhookInstance(t, tc)
	// Whitespace deliberately differs from a marshal round trip; verification must use original bytes.
	body := "{\n  \"id\": \"evt_raw_signed\", \"object\": \"event\", \"type\": \"customer.subscription.updated\",\n" +
		"  \"livemode\": false, \"data\": {\"object\": {\"customer\": \"cus_unlinked\"}}\n}"
	invalid := sendStripeContractEvent(t, tc.product.ProductID, body, "t=1,v1=invalid")
	require.Equal(t, http.StatusBadRequest, invalid.StatusCode(), string(invalid.Body))
	requireNoStripeBillingState(t, instance)
	for _, suffix := range []string{`, "livemode": true`, `, "account": "acct_Foreign123"`} {
		unsafeBody := `{"id":"evt_refused","object":"event","type":"customer.subscription.updated",` +
			`"data":{"object":{"customer":"cus_unlinked"}}` + suffix + `}`
		response := sendStripeContractEvent(t, tc.product.ProductID, unsafeBody, signedStripeContractEvent(unsafeBody))
		require.Equal(t, http.StatusBadRequest, response.StatusCode(), string(response.Body))
		requireNoStripeBillingState(t, instance)
	}
	for range 2 {
		response := sendStripeContractEvent(t, tc.product.ProductID, body, signedStripeContractEvent(body))
		require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	}
	events := storedStripeEvents(t, instance)
	require.Len(t, events, 1)
	var event billing.BillingEvent
	require.NoError(t, json.Unmarshal(events["evt_raw_signed"], &event))
	require.Equal(t, "ignored", event.Status)
	require.Empty(t, event.OrganizationID)
}

func seedLinkedStripeState(t *testing.T, tc webhookSecretTestCtx, instance integration.Instance) string {
	t.Helper()
	organization := tc.product.CreateOrganization(t, "Webhook transaction fixture", nil)
	body, err := json.Marshal(billing.NewStoredState(stripeFixtureAccount, tc.product.ProductID))
	require.NoError(t, err)
	var state map[string]any
	require.NoError(t, json.Unmarshal(body, &state))
	state["organizations"] = map[string]any{organization.Id: map[string]any{
		"id": organization.Id, "customer_id": "cus_linked_fixture", "name": organization.Name,
	}}
	body, err = json.Marshal(state)
	require.NoError(t, err)
	_, err = testDB.ExecContext(t.Context(), `INSERT INTO stripe_billing_states
		(integration_instance_id,platform_tenant_id,product_id,state_json) VALUES($1,$2,$3,$4::jsonb)`,
		instance.ID, tc.tenantID, tc.product.ProductID, string(body))
	require.NoError(t, err)
	return organization.Id
}

// Defer only this fixture's jobs, so the CT can inspect persistence without invoking Stripe.
// The integration is removed before the deferral trigger during cleanup.
func deferStripeFixtureJobs(t *testing.T, instanceID string) {
	t.Helper()
	name := pq.QuoteIdentifier("stripe_ct_" + instanceID)
	_, err := testDB.ExecContext(t.Context(), fmt.Sprintf(`CREATE FUNCTION %s() RETURNS TRIGGER AS $$
		BEGIN
			IF NEW.queue_name = 'stripe-billing-events' AND
			   convert_from(NEW.payload,'UTF8')::jsonb ->> 'integration_instance_id' = TG_ARGV[0] THEN
				NEW.available_at := NOW() + INTERVAL '1 day';
			END IF;
			RETURN NEW;
		END; $$ LANGUAGE plpgsql`, name))
	require.NoError(t, err)
	_, err = testDB.ExecContext(t.Context(), fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON pgqueue_jobs
		FOR EACH ROW EXECUTE FUNCTION %s(%s)`, name, name, pq.QuoteLiteral(instanceID)))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, dropErr := testDB.ExecContext(
			context.Background(),
			fmt.Sprintf(`DROP FUNCTION IF EXISTS %s() CASCADE`, name),
		)
		require.NoError(t, dropErr)
	})
}

func countStripeFixtureJobs(t *testing.T, instanceID string) int {
	t.Helper()
	var count int
	require.NoError(t, testDB.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM pgqueue_jobs
		WHERE queue_name=$1 AND convert_from(payload,'UTF8')::jsonb ->> 'integration_instance_id'=$2`,
		stripebilling.EventsQueueName, instanceID).Scan(&count))
	return count
}

func TestStripeWebhookCommitsStateAndDurableQueueTogether(t *testing.T) {
	t.Parallel()
	tc := newWebhookSecretTestCtx(t)
	// Cleanup runs in reverse order: remove integration before dropping its queue deferral.
	instance := seedActiveStripeWebhookInstance(t, tc)
	deferStripeFixtureJobs(t, instance.ID)
	t.Cleanup(func() {
		require.NoError(t, IntegrationRepo.DeleteByID(context.Background(), tc.tenantID, instance.ID))
	})
	organizationID := seedLinkedStripeState(t, tc, instance)
	name := pq.QuoteIdentifier("stripe_queue_failure_" + instance.ID)
	_, err := testDB.ExecContext(t.Context(), fmt.Sprintf(`ALTER TABLE pgqueue_jobs ADD CONSTRAINT %s CHECK (
		queue_name <> 'stripe-billing-events' OR
		convert_from(payload,'UTF8')::jsonb ->> 'integration_instance_id' IS DISTINCT FROM %s)`,
		name, pq.QuoteLiteral(instance.ID)))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, dropErr := testDB.ExecContext(
			context.Background(),
			fmt.Sprintf(`ALTER TABLE pgqueue_jobs DROP CONSTRAINT IF EXISTS %s`, name),
		)
		require.NoError(t, dropErr)
	})
	body := `{"id":"evt_transactional","object":"event","type":"customer.subscription.updated",` +
		`"livemode":false,"data":{"object":{"customer":"cus_linked_fixture"}}}`
	failed := sendStripeContractEvent(t, tc.product.ProductID, body, signedStripeContractEvent(body))
	require.Equal(t, http.StatusInternalServerError, failed.StatusCode(), string(failed.Body))
	require.Empty(t, storedStripeEvents(t, instance), "event state must roll back if enqueue fails")
	require.Zero(t, countStripeFixtureJobs(t, instance.ID))
	_, err = testDB.ExecContext(t.Context(), fmt.Sprintf(`ALTER TABLE pgqueue_jobs DROP CONSTRAINT %s`, name))
	require.NoError(t, err)
	for range 2 {
		response := sendStripeContractEvent(t, tc.product.ProductID, body, signedStripeContractEvent(body))
		require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	}
	events := storedStripeEvents(t, instance)
	require.Len(t, events, 1)
	var event billing.BillingEvent
	require.NoError(t, json.Unmarshal(events["evt_transactional"], &event))
	require.Equal(t, "pending", event.Status)
	require.Equal(t, organizationID, event.OrganizationID)
	require.Equal(t, 1, countStripeFixtureJobs(t, instance.ID), "duplicate delivery must not enqueue another job")
	var available time.Time
	require.NoError(t, testDB.QueryRowContext(t.Context(), `SELECT available_at FROM pgqueue_jobs
		WHERE queue_name=$1 AND convert_from(payload,'UTF8')::jsonb ->> 'integration_instance_id'=$2`,
		stripebilling.EventsQueueName, instance.ID).Scan(&available))
	require.True(t, available.After(time.Now()), "fixture jobs remain private to database verification")
}

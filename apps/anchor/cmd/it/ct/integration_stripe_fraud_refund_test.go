package ct_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"anchor/internal/domain/integration"
	"anchor/internal/stripebilling/billing"

	"github.com/lib/pq"
	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
)

func readStripeFinancialState(t *testing.T, instance integration.Instance) billing.StoredState {
	t.Helper()
	var raw []byte
	require.NoError(t, testDB.QueryRowContext(t.Context(), `SELECT state_json FROM stripe_billing_states
		WHERE integration_instance_id=$1 AND platform_tenant_id=$2 AND product_id=$3`,
		instance.ID, instance.PlatformTenantID, instance.ProductID).Scan(&raw))
	var state billing.StoredState
	require.NoError(t, json.Unmarshal(raw, &state))
	return state
}

func TestStripeFraudRefundSettingsValidateAndPreserveUnchangedFields(t *testing.T) {
	t.Parallel()
	tc := newWebhookSecretTestCtx(t)
	instance := seedActiveStripeWebhookInstance(t, tc)
	deferStripeFixtureJobs(t, instance.ID)
	seedLinkedStripeState(t, tc, instance)
	client := tc.product.OwnerAuthenticatedClient()
	schema, err := client.CreateLicenseSchemaWithResponse(
		t.Context(),
		tc.product.ProductID,
		ct.CreateLicenseSchemaJSONRequestBody{
			Fields: []ct.LicenseFieldDeclaration{{Name: "fraud_fixture", Type: ct.LicenseFieldTypeBOOLEAN}},
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, schema.StatusCode(), string(schema.Body))
	template, err := client.CreateLicenseTemplateWithResponse(
		t.Context(),
		tc.product.ProductID,
		ct.CreateLicenseTemplateJSONRequestBody{
			Name:   "Refund fallback " + ksuid.New().String(),
			Values: ct.LicenseTemplateValues{"fraud_fixture": false},
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, template.StatusCode(), string(template.Body))
	require.NotNil(t, template.JSON201)
	for name, request := range map[string]ct.UpdateStripeBillingSettingsJSONRequestBody{
		"no fields":            {},
		"enabled zero maximum": {FraudRefundPolicy: &ct.StripeBillingFraudRefundPolicy{Enabled: true, Currency: ct.StripeBillingFraudRefundCurrencyUsd}},
		"negative amount":      {FraudRefundPolicy: &ct.StripeBillingFraudRefundPolicy{Currency: ct.StripeBillingFraudRefundCurrencyUsd, MaxAmount: -1}},
		"unsupported currency": {FraudRefundPolicy: &ct.StripeBillingFraudRefundPolicy{Currency: "gbp"}},
	} {
		t.Run(name, func(t *testing.T) {
			response, updateErr := client.UpdateStripeBillingSettingsWithResponse(
				t.Context(),
				tc.product.ProductID,
				request,
			)
			require.NoError(t, updateErr)
			require.Equal(t, http.StatusBadRequest, response.StatusCode(), string(response.Body))
		})
	}
	policy := ct.StripeBillingFraudRefundPolicy{
		Enabled:   true,
		Currency:  ct.StripeBillingFraudRefundCurrencyCad,
		MaxAmount: 1500,
	}
	response, err := client.UpdateStripeBillingSettingsWithResponse(t.Context(), tc.product.ProductID,
		ct.UpdateStripeBillingSettingsJSONRequestBody{FraudRefundPolicy: &policy})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	require.NotNil(t, response.JSON200)
	require.Equal(t, policy, response.JSON200.FraudRefundPolicy)
	response, err = client.UpdateStripeBillingSettingsWithResponse(t.Context(), tc.product.ProductID,
		ct.UpdateStripeBillingSettingsJSONRequestBody{FallbackTemplateId: &template.JSON201.Id})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	require.Equal(t, policy, response.JSON200.FraudRefundPolicy)
	policy.Enabled, policy.MaxAmount = false, 0
	response, err = client.UpdateStripeBillingSettingsWithResponse(t.Context(), tc.product.ProductID,
		ct.UpdateStripeBillingSettingsJSONRequestBody{FraudRefundPolicy: &policy})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	require.Equal(t, template.JSON201.Id, response.JSON200.FallbackTemplateId)
	stored := readStripeFinancialState(t, instance)
	require.False(t, stored.Settings.FraudRefundPolicy.Enabled)
	require.Equal(t, billing.FraudRefundCurrencyCad, stored.Settings.FraudRefundPolicy.Currency)
	require.Equal(t, template.JSON201.Id, stored.Settings.FallbackTemplateID)
}

func TestStripeFraudWarningDefaultOffAuditsWithoutExternalCallsAndNeverBackfills(t *testing.T) {
	t.Parallel()
	tc := newWebhookSecretTestCtx(t)
	instance := seedActiveStripeWebhookInstance(t, tc)
	seedLinkedStripeState(t, tc, instance)
	body := `{"id":"evt_fraud_off","object":"event","type":"radar.early_fraud_warning.created",` +
		`"livemode":false,"data":{"object":{"id":"issfr_off","charge":"ch_off","actionable":true}}}`
	for range 2 {
		response := sendStripeContractEvent(t, tc.product.ProductID, body, signedStripeContractEvent(body))
		require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	}
	require.Eventually(t, func() bool {
		var reason string
		err := testDB.QueryRowContext(t.Context(), `SELECT state_json -> 'fraud_refunds' -> 'ch_off' ->> 'reason'
			FROM stripe_billing_states WHERE integration_instance_id=$1 AND platform_tenant_id=$2 AND product_id=$3`,
			instance.ID, instance.PlatformTenantID, instance.ProductID).Scan(&reason)
		return err == nil && reason == "policy_disabled"
	}, 10*time.Second, 10*time.Millisecond)
	policy := ct.StripeBillingFraudRefundPolicy{
		Enabled:   true,
		Currency:  ct.StripeBillingFraudRefundCurrencyUsd,
		MaxAmount: 1500,
	}
	response, err := tc.product.OwnerAuthenticatedClient().
		UpdateStripeBillingSettingsWithResponse(t.Context(), tc.product.ProductID,
			ct.UpdateStripeBillingSettingsJSONRequestBody{FraudRefundPolicy: &policy})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	updated := `{"id":"evt_fraud_updated","object":"event","type":"radar.early_fraud_warning.updated",` +
		`"livemode":false,"data":{"object":{"id":"issfr_off","charge":"ch_off","actionable":true}}}`
	received := sendStripeContractEvent(t, tc.product.ProductID, updated, signedStripeContractEvent(updated))
	require.Equal(t, http.StatusOK, received.StatusCode(), string(received.Body))
	require.Eventually(t, func() bool {
		var status string
		queryErr := testDB.QueryRowContext(t.Context(), `SELECT state_json -> 'events' -> 'evt_fraud_updated' ->> 'status'
			FROM stripe_billing_states WHERE integration_instance_id=$1 AND platform_tenant_id=$2 AND product_id=$3`,
			instance.ID, instance.PlatformTenantID, instance.ProductID).
			Scan(&status)
		return queryErr == nil && status == "processed"
	}, 10*time.Second, 10*time.Millisecond)
	stored := readStripeFinancialState(t, instance)
	encoded, err := json.Marshal(stored)
	require.NoError(t, err)
	var public struct {
		Refunds map[string]billing.FraudRefund `json:"fraud_refunds"`
	}
	require.NoError(t, json.Unmarshal(encoded, &public))
	require.Len(t, public.Refunds, 1)
	require.Equal(t, billing.FraudRefundSkipped, public.Refunds["ch_off"].Status)
	require.Empty(t, public.Refunds["ch_off"].RefundID)
	require.NotContains(t, string(encoded), stripeFixtureKey)
	require.NotContains(t, string(encoded), stripeFixtureWebhook)
}

func TestStripeFraudWarningIntakeAndDurableQueueCommitAtomically(t *testing.T) {
	t.Parallel()
	tc := newWebhookSecretTestCtx(t)
	instance := seedActiveStripeWebhookInstance(t, tc)
	deferStripeFixtureJobs(t, instance.ID)
	seedLinkedStripeState(t, tc, instance)
	policy := ct.StripeBillingFraudRefundPolicy{
		Enabled:   true,
		Currency:  ct.StripeBillingFraudRefundCurrencyUsd,
		MaxAmount: 1500,
	}
	response, err := tc.product.OwnerAuthenticatedClient().
		UpdateStripeBillingSettingsWithResponse(t.Context(), tc.product.ProductID,
			ct.UpdateStripeBillingSettingsJSONRequestBody{FraudRefundPolicy: &policy})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	name := pq.QuoteIdentifier("stripe_fraud_queue_failure_" + instance.ID)
	_, err = testDB.ExecContext(t.Context(), fmt.Sprintf(`ALTER TABLE pgqueue_jobs ADD CONSTRAINT %s CHECK (
		queue_name <> 'stripe-billing-events' OR
		convert_from(payload,'UTF8')::jsonb ->> 'integration_instance_id' IS DISTINCT FROM %s)`, name, pq.QuoteLiteral(instance.ID)))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, dropErr := testDB.ExecContext(
			context.Background(),
			fmt.Sprintf(`ALTER TABLE pgqueue_jobs DROP CONSTRAINT IF EXISTS %s`, name),
		)
		require.NoError(t, dropErr)
	})
	body := `{"id":"evt_fraud_transaction","object":"event","type":"radar.early_fraud_warning.created",` +
		`"livemode":false,"data":{"object":{"id":"issfr_transaction","charge":"ch_transaction","actionable":true}}}`
	failed := sendStripeContractEvent(t, tc.product.ProductID, body, signedStripeContractEvent(body))
	require.Equal(t, http.StatusInternalServerError, failed.StatusCode(), string(failed.Body))
	require.Empty(t, storedStripeEvents(t, instance))
	require.Zero(t, countStripeFixtureJobs(t, instance.ID))
	_, err = testDB.ExecContext(t.Context(), fmt.Sprintf(`ALTER TABLE pgqueue_jobs DROP CONSTRAINT %s`, name))
	require.NoError(t, err)
	for range 2 {
		received := sendStripeContractEvent(t, tc.product.ProductID, body, signedStripeContractEvent(body))
		require.Equal(t, http.StatusOK, received.StatusCode(), string(received.Body))
	}
	events := storedStripeEvents(t, instance)
	require.Len(t, events, 1)
	var event struct {
		billing.BillingEvent
		ResourceID    string `json:"resource_id"`
		ChargeID      string `json:"charge_id"`
		PolicyEnabled bool   `json:"policy_enabled"`
	}
	require.NoError(t, json.Unmarshal(events["evt_fraud_transaction"], &event))
	require.Equal(t, "pending", event.Status)
	require.Equal(t, "issfr_transaction", event.ResourceID)
	require.Equal(t, "ch_transaction", event.ChargeID)
	require.True(t, event.PolicyEnabled)
	require.Equal(t, 1, countStripeFixtureJobs(t, instance.ID))
}

func TestStripeConcurrentFinancialStateWritesPreserveWebhookAndSettings(t *testing.T) {
	// Not parallel: verifies two state writers blocked on the same owned JSONB row.
	tc := newWebhookSecretTestCtx(t)
	instance := seedActiveStripeWebhookInstance(t, tc)
	deferStripeFixtureJobs(t, instance.ID)
	seedLinkedStripeState(t, tc, instance)
	tx, err := testDB.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	var pid int
	require.NoError(t, tx.QueryRowContext(t.Context(), "SELECT pg_backend_pid()").Scan(&pid))
	var instanceID string
	require.NoError(t, tx.QueryRowContext(t.Context(), `SELECT integration_instance_id FROM stripe_billing_states
		WHERE integration_instance_id=$1 AND platform_tenant_id=$2 AND product_id=$3 FOR UPDATE`,
		instance.ID, instance.PlatformTenantID, instance.ProductID).Scan(&instanceID))
	policy := ct.StripeBillingFraudRefundPolicy{
		Enabled: true, Currency: ct.StripeBillingFraudRefundCurrencyCad, MaxAmount: 1250,
	}
	settingsDone := make(chan error, 1)
	client := tc.product.OwnerAuthenticatedClient()
	go func() {
		response, updateErr := client.UpdateStripeBillingSettingsWithResponse(t.Context(), tc.product.ProductID,
			ct.UpdateStripeBillingSettingsJSONRequestBody{FraudRefundPolicy: &policy})
		if updateErr == nil && response.StatusCode() != http.StatusOK {
			updateErr = fmt.Errorf("settings returned HTTP %d", response.StatusCode())
		}
		settingsDone <- updateErr
	}()
	webhookDone := waitingStripeWebhook(t, tc.product.ProductID, http.StatusOK)
	var premature bool
	var requestErr error
	require.Eventually(t, func() bool {
		for _, completed := range []<-chan error{settingsDone, webhookDone} {
			select {
			case requestErr = <-completed:
				premature = true
				return true
			default:
			}
		}
		var waiting int
		queryErr := testDB.QueryRowContext(t.Context(), `WITH RECURSIVE blocked(pid) AS (
			SELECT pid FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid))
			UNION SELECT activity.pid FROM pg_stat_activity activity JOIN blocked
			ON blocked.pid=ANY(pg_blocking_pids(activity.pid))) SELECT count(*) FROM blocked`, pid).Scan(&waiting)
		return queryErr == nil && waiting >= 2
	}, 5*time.Second, 10*time.Millisecond)
	require.False(t, premature, "state writes must wait for the owned row: %v", requestErr)
	require.NoError(t, tx.Commit())
	awaitStripeMutation(t, settingsDone)
	awaitStripeMutation(t, webhookDone)
	stored := readStripeFinancialState(t, instance)
	require.True(t, stored.Settings.FraudRefundPolicy.Enabled)
	require.Equal(t, billing.FraudRefundCurrencyCad, stored.Settings.FraudRefundPolicy.Currency)
	require.Equal(t, int64(1250), stored.Settings.FraudRefundPolicy.MaxAmount)
	events := storedStripeEvents(t, instance)
	require.Len(t, events, 1)
	require.Contains(t, events, "evt_ingress")
	require.Equal(t, 1, countStripeFixtureJobs(t, instance.ID))
}

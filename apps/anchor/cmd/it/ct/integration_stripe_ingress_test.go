package ct_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	itshared "anchor/cmd/it/shared"
	dslfactory "anchor/cmd/it/shared/dsl/factory"
	"anchor/internal/domain/integration"
	stripeprovider "anchor/internal/integration/provider/stripe"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

const stripeIngressBody = `{"id":"evt_ingress","object":"event","type":"radar.early_fraud_warning.created",` +
	`"livemode":false,"data":{"object":{"id":"issfr_ingress","charge":"ch_ingress","actionable":true}}}`

func TestStripeWebhookIntakePersistsWhileFinancialOperationRuns(t *testing.T) {
	// Not parallel: verifies webhook receipt while its exact financial advisory lock is held.
	tc := newWebhookSecretTestCtx(t)
	instance := seedActiveStripeWebhookInstance(t, tc)
	deferStripeFixtureJobs(t, instance.ID)
	seedLinkedStripeState(t, tc, instance)
	policy := ct.StripeBillingFraudRefundPolicy{
		Enabled: true, Currency: ct.StripeBillingFraudRefundCurrencyUsd, MaxAmount: 1500,
	}
	settings, err := tc.product.OwnerAuthenticatedClient().UpdateStripeBillingSettingsWithResponse(
		t.Context(), tc.product.ProductID, ct.UpdateStripeBillingSettingsJSONRequestBody{FraudRefundPolicy: &policy},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, settings.StatusCode(), string(settings.Body))
	_, release := holdStripeBillingOperation(t, instance.ID)
	defer release()
	for range 2 {
		response := sendStripeContractEvent(
			t,
			tc.product.ProductID,
			stripeIngressBody,
			signedStripeContractEvent(stripeIngressBody),
		)
		require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	}
	events := storedStripeEvents(t, instance)
	require.Len(t, events, 1)
	var event struct {
		PolicyEnabled bool   `json:"policy_enabled"`
		ChargeID      string `json:"charge_id"`
		Status        string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(events["evt_ingress"], &event))
	require.True(t, event.PolicyEnabled)
	require.Equal(t, "ch_ingress", event.ChargeID)
	require.Equal(t, "pending", event.Status)
	require.Equal(t, 1, countStripeFixtureJobs(t, instance.ID))
}

func heldStripeInstanceTransaction(t *testing.T, instance integration.Instance) (*sql.Tx, int) {
	t.Helper()
	tx, err := testDB.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	var pid int
	require.NoError(t, tx.QueryRowContext(t.Context(), "SELECT pg_backend_pid()").Scan(&pid))
	var id string
	require.NoError(t, tx.QueryRowContext(t.Context(), `SELECT id FROM integration_instances
		WHERE id=$1 AND platform_tenant_id=$2 AND product_id=$3 FOR UPDATE`,
		instance.ID, instance.PlatformTenantID, instance.ProductID).Scan(&id))
	return tx, pid
}

func waitingStripeWebhook(t *testing.T, productID string, expectedStatus int) <-chan error {
	t.Helper()
	completed := make(chan error, 1)
	client := dslfactory.NewNoAuthClient(t, itshared.ServerURL)
	go func() {
		response, err := client.IngestStripeBillingWebhookWithBodyWithResponse(t.Context(), productID,
			&ct.IngestStripeBillingWebhookParams{StripeSignature: signedStripeContractEvent(stripeIngressBody)},
			"application/json", strings.NewReader(stripeIngressBody))
		if err == nil && response.StatusCode() != expectedStatus {
			err = fmt.Errorf("webhook returned HTTP %d, expected %d", response.StatusCode(), expectedStatus)
		}
		completed <- err
	}()
	return completed
}

func waitForStripeWebhookRowLock(t *testing.T, pid int, completed <-chan error) {
	t.Helper()
	var premature bool
	var requestErr error
	require.Eventually(t, func() bool {
		select {
		case requestErr = <-completed:
			premature = true
			return true
		default:
		}
		var waiting int
		err := testDB.QueryRowContext(t.Context(), `SELECT count(*) FROM pg_stat_activity
			WHERE wait_event_type='Lock' AND $1=ANY(pg_blocking_pids(pid))`, pid).Scan(&waiting)
		return err == nil && waiting > 0
	}, 5*time.Second, 10*time.Millisecond)
	require.False(t, premature, "webhook must wait for the current integration configuration: %v", requestErr)
}

func TestStripeWebhookIntakeReloadsRotatedSecretBeforeReceipt(t *testing.T) {
	// Not parallel: verifies webhook signature against configuration committed during a row-lock wait.
	tc := newWebhookSecretTestCtx(t)
	instance := seedActiveStripeWebhookInstance(t, tc)
	deferStripeFixtureJobs(t, instance.ID)
	seedLinkedStripeState(t, tc, instance)
	config, err := json.Marshal(stripeprovider.Config{
		AuthMethod: stripeprovider.AuthMethodAPIKey, AccountID: stripeFixtureAccount,
		APIKey: stripeFixtureKey, WebhookSecret: "whsec_rotated_fixture", ReturnURL: "https://anchor.example.test",
	})
	require.NoError(t, err)
	stored, err := StripeProvider.PrepareConfigForStorage(t.Context(), config)
	require.NoError(t, err)
	tx, pid := heldStripeInstanceTransaction(t, instance)
	_, err = tx.ExecContext(t.Context(), `UPDATE integration_instances SET config_json=$1::jsonb
		WHERE id=$2 AND platform_tenant_id=$3 AND product_id=$4`, string(stored), instance.ID, tc.tenantID, tc.product.ProductID)
	require.NoError(t, err)
	completed := waitingStripeWebhook(t, tc.product.ProductID, http.StatusBadRequest)
	waitForStripeWebhookRowLock(t, pid, completed)
	require.NoError(t, tx.Commit())
	awaitStripeMutation(t, completed)
	require.Empty(t, storedStripeEvents(t, instance))
	require.Zero(t, countStripeFixtureJobs(t, instance.ID))
}

func TestStripeWebhookIntakeReloadsDisabledConnectionBeforeReceipt(t *testing.T) {
	// Not parallel: verifies active connection scope committed during a row-lock wait.
	tc := newWebhookSecretTestCtx(t)
	instance := seedActiveStripeWebhookInstance(t, tc)
	deferStripeFixtureJobs(t, instance.ID)
	seedLinkedStripeState(t, tc, instance)
	tx, pid := heldStripeInstanceTransaction(t, instance)
	_, err := tx.ExecContext(t.Context(), `UPDATE integration_instances SET is_enabled=false, status='INACTIVE'
		WHERE id=$1 AND platform_tenant_id=$2 AND product_id=$3`, instance.ID, tc.tenantID, tc.product.ProductID)
	require.NoError(t, err)
	completed := waitingStripeWebhook(t, tc.product.ProductID, http.StatusConflict)
	waitForStripeWebhookRowLock(t, pid, completed)
	require.NoError(t, tx.Commit())
	awaitStripeMutation(t, completed)
	require.Empty(t, storedStripeEvents(t, instance))
	require.Zero(t, countStripeFixtureJobs(t, instance.ID))
}

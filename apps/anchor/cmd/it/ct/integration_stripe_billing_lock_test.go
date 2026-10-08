package ct_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"anchor/internal/stripebilling/billing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

func holdStripeBillingOperation(t *testing.T, instanceID string) (int, func()) {
	t.Helper()
	connection, err := testDB.Conn(t.Context())
	require.NoError(t, err)
	key := "stripe-billing:" + instanceID
	_, err = connection.ExecContext(t.Context(), "SELECT pg_advisory_lock(hashtextextended($1,0))", key)
	require.NoError(t, err)
	var pid int
	require.NoError(t, connection.QueryRowContext(t.Context(), "SELECT pg_backend_pid()").Scan(&pid))
	var once sync.Once
	release := func() {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, unlockErr := connection.ExecContext(ctx, "SELECT pg_advisory_unlock(hashtextextended($1,0))", key)
			closeErr := connection.Close()
			require.NoError(t, unlockErr)
			require.NoError(t, closeErr)
		})
	}
	t.Cleanup(release)
	return pid, release
}

func waitForStripeMutationLock(t *testing.T, pid int, completed ...<-chan error) {
	t.Helper()
	var premature bool
	var mutationErr error
	require.Eventually(t, func() bool {
		for _, result := range completed {
			select {
			case mutationErr = <-result:
				premature = true
				return true
			default:
			}
		}
		var waiting int
		err := testDB.QueryRowContext(t.Context(), `SELECT count(*) FROM pg_locks
			WHERE locktype='advisory' AND NOT granted AND $1=ANY(pg_blocking_pids(pid))`, pid).Scan(&waiting)
		return err == nil && waiting >= len(completed)
	}, 5*time.Second, 10*time.Millisecond, "integration mutation must wait for the billing operation lock")
	require.False(t, premature, "integration mutation returned before billing operation completed: %v", mutationErr)
}

func awaitStripeMutation(t *testing.T, completed <-chan error) {
	t.Helper()
	select {
	case err := <-completed:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("integration mutation did not complete after billing operation released its lock")
	}
}

func TestStripeIntegrationDisableWaitsForBillingOperation(t *testing.T) {
	// Not parallel: verifies database advisory-lock timing through the public mutation API.
	tc := newWebhookSecretTestCtx(t)
	instance := createStripeContractInstance(t, tc)
	pid, release := holdStripeBillingOperation(t, instance.Id)
	completed := make(chan error, 1)
	client := tc.product.OwnerAuthenticatedClient()
	go func() {
		response, err := client.UpdateIntegrationInstanceWithResponse(t.Context(), tc.product.ProductID,
			instance.Id, ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(false)})
		if err == nil && response.StatusCode() != http.StatusOK {
			err = fmt.Errorf("disable returned HTTP %d", response.StatusCode())
		}
		completed <- err
	}()
	waitForStripeMutationLock(t, pid, completed)
	before := getIntegrationInstance(t, tc.product, instance.Id)
	require.True(t, before.IsEnabled)
	release()
	awaitStripeMutation(t, completed)
	after := getIntegrationInstance(t, tc.product, instance.Id)
	require.False(t, after.IsEnabled)
}

func seedStripeRefundAudit(t *testing.T, tc webhookSecretTestCtx, instanceID, status string) {
	t.Helper()
	state := billing.NewStoredState(stripeFixtureAccount, tc.product.ProductID)
	encoded, err := json.Marshal(state)
	require.NoError(t, err)
	var value map[string]any
	require.NoError(t, json.Unmarshal(encoded, &value))
	value["fraud_refunds"] = map[string]any{
		"ch_lock_fixture": map[string]any{
			"id": "ffr_lock_fixture", "charge_id": "ch_lock_fixture", "status": status,
			"amount": 100, "currency": "usd", "installation_id": state.InstallationID,
			"product_id": state.ProductID,
		},
	}
	encoded, err = json.Marshal(value)
	require.NoError(t, err)
	_, err = testDB.ExecContext(t.Context(), `INSERT INTO stripe_billing_states
		(integration_instance_id, platform_tenant_id, product_id, state_json) VALUES ($1,$2,$3,$4::jsonb)`,
		instanceID, tc.tenantID, tc.product.ProductID, string(encoded))
	require.NoError(t, err)
}

func TestStripeIntegrationRefusesDeletingUnresolvedRefunds(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"processing", "pending", "requires_action", "review_required", "unknown"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			tc := newWebhookSecretTestCtx(t)
			instance := createStripeContractInstance(t, tc)
			seedStripeRefundAudit(t, tc, instance.Id, status)
			response, err := tc.product.OwnerAuthenticatedClient().DeleteIntegrationInstanceWithResponse(
				t.Context(), tc.product.ProductID, instance.Id)
			require.NoError(t, err)
			require.Equal(t, http.StatusConflict, response.StatusCode())
			require.Contains(t, string(response.Body), "Resolve pending refunds before disconnecting Stripe")
			retained := getIntegrationInstance(t, tc.product, instance.Id)
			require.Equal(t, instance.Id, retained.Id)
		})
	}
}

func TestStripeIntegrationDeletesResolvedRefunds(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"succeeded", "skipped", "failed", "canceled"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			tc := newWebhookSecretTestCtx(t)
			instance := createStripeContractInstance(t, tc)
			seedStripeRefundAudit(t, tc, instance.Id, status)
			client := tc.product.OwnerAuthenticatedClient()
			response, err := client.DeleteIntegrationInstanceWithResponse(
				t.Context(),
				tc.product.ProductID,
				instance.Id,
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, response.StatusCode())
			removed, err := client.GetIntegrationInstanceWithResponse(t.Context(), tc.product.ProductID, instance.Id)
			require.NoError(t, err)
			require.Equal(t, http.StatusNotFound, removed.StatusCode())
		})
	}
}

func TestStripeIntegrationCanDisableWithUnresolvedRefunds(t *testing.T) {
	t.Parallel()
	tc := newWebhookSecretTestCtx(t)
	instance := createStripeContractInstance(t, tc)
	seedStripeRefundAudit(t, tc, instance.Id, "pending")
	response := updateInstance(t, tc, instance.Id, ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(false)})
	require.Equal(t, http.StatusOK, response.StatusCode())
	require.False(t, response.JSON200.IsEnabled)
	deletion, err := tc.product.OwnerAuthenticatedClient().DeleteIntegrationInstanceWithResponse(
		t.Context(), tc.product.ProductID, instance.Id)
	require.NoError(t, err)
	require.Equal(
		t,
		http.StatusConflict,
		deletion.StatusCode(),
		"disabling must preserve the unresolved refund receipt",
	)
}

func TestStripeIntegrationRefusesDeletingCorruptBillingState(t *testing.T) {
	t.Parallel()
	for _, invalid := range []string{`"malformed"`, `null`, `{}`} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()
			tc := newWebhookSecretTestCtx(t)
			instance := createStripeContractInstance(t, tc)
			_, err := testDB.ExecContext(t.Context(), `INSERT INTO stripe_billing_states
				(integration_instance_id,platform_tenant_id,product_id,state_json) VALUES ($1,$2,$3,$4::jsonb)`,
				instance.Id, tc.tenantID, tc.product.ProductID, invalid)
			require.NoError(t, err)
			response, err := tc.product.OwnerAuthenticatedClient().DeleteIntegrationInstanceWithResponse(
				t.Context(), tc.product.ProductID, instance.Id)
			require.NoError(t, err)
			require.Equal(t, http.StatusConflict, response.StatusCode())
		})
	}
}

func TestStripeIntegrationDeleteWaitsForBillingOperation(t *testing.T) {
	// Not parallel: verifies database advisory-lock timing through the public mutation API.
	tc := newWebhookSecretTestCtx(t)
	instance := createStripeContractInstance(t, tc)
	pid, release := holdStripeBillingOperation(t, instance.Id)
	completed := make(chan error, 1)
	client := tc.product.OwnerAuthenticatedClient()
	go func() {
		response, err := client.DeleteIntegrationInstanceWithResponse(t.Context(), tc.product.ProductID, instance.Id)
		if err == nil && response.StatusCode() != http.StatusNoContent {
			err = fmt.Errorf("delete returned HTTP %d", response.StatusCode())
		}
		completed <- err
	}()
	waitForStripeMutationLock(t, pid, completed)
	require.Equal(t, instance.Id, getIntegrationInstance(t, tc.product, instance.Id).Id)
	release()
	awaitStripeMutation(t, completed)
	removed, err := client.GetIntegrationInstanceWithResponse(t.Context(), tc.product.ProductID, instance.Id)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, removed.StatusCode())
}

func TestStripeIntegrationConcurrentChangesMergeAfterBillingOperation(t *testing.T) {
	// Not parallel: verifies database advisory-lock timing and post-lock configuration merging.
	tc := newWebhookSecretTestCtx(t)
	instance := createStripeContractInstance(t, tc)
	pid, release := holdStripeBillingOperation(t, instance.Id)
	client := tc.product.OwnerAuthenticatedClient()
	rotated := ct.IntegrationProviderConfig{}
	account := "acct_RotatedFixture"
	require.NoError(t, rotated.FromStripeIntegrationConfig(ct.StripeIntegrationConfig{AccountId: &account}))
	configuration := make(chan error, 1)
	go func() {
		response, err := client.UpdateIntegrationInstanceWithResponse(t.Context(), tc.product.ProductID,
			instance.Id, ct.UpdateIntegrationInstanceJSONRequestBody{Config: &rotated})
		if err == nil && response.StatusCode() != http.StatusOK {
			err = fmt.Errorf("configuration update returned HTTP %d", response.StatusCode())
		}
		configuration <- err
	}()
	waitForStripeMutationLock(t, pid, configuration)
	disable := make(chan error, 1)
	go func() {
		response, err := client.UpdateIntegrationInstanceWithResponse(t.Context(), tc.product.ProductID,
			instance.Id, ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(false)})
		if err == nil && response.StatusCode() != http.StatusOK {
			err = fmt.Errorf("disable returned HTTP %d", response.StatusCode())
		}
		disable <- err
	}()
	waitForStripeMutationLock(t, pid, configuration, disable)
	release()
	awaitStripeMutation(t, configuration)
	awaitStripeMutation(t, disable)
	after := getIntegrationInstance(t, tc.product, instance.Id)
	public, err := after.PublicConfig.AsStripeIntegrationPublicConfig()
	require.NoError(t, err)
	require.Equal(t, account, *public.AccountId)
	require.False(t, after.IsEnabled)
}

func TestStripeIntegrationCanceledMutationLeavesConfigurationUnchanged(t *testing.T) {
	// Not parallel: verifies cancellation while waiting for the billing advisory lock.
	tc := newWebhookSecretTestCtx(t)
	instance := createStripeContractInstance(t, tc)
	pid, release := holdStripeBillingOperation(t, instance.Id)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	completed := make(chan error, 1)
	client := tc.product.OwnerAuthenticatedClient()
	go func() {
		_, err := client.UpdateIntegrationInstanceWithResponse(ctx,
			tc.product.ProductID, instance.Id, ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(false)})
		completed <- err
	}()
	waitForStripeMutationLock(t, pid, completed)
	cancel()
	select {
	case err := <-completed:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("canceled mutation did not exit")
	}
	require.Eventually(t, func() bool {
		var waiting int
		err := testDB.QueryRowContext(t.Context(), `SELECT count(*) FROM pg_locks
			WHERE locktype='advisory' AND NOT granted AND $1=ANY(pg_blocking_pids(pid))`, pid).Scan(&waiting)
		return err == nil && waiting == 0
	}, 5*time.Second, 10*time.Millisecond)
	release()
	require.True(t, getIntegrationInstance(t, tc.product, instance.Id).IsEnabled)
}

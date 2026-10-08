//nolint:testpackage // Recovery tests inspect persisted financial intents across service restarts.
package billing

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type refundWorld struct {
	*serviceWorld
	warning      map[string]any
	charge       map[string]any
	payment      map[string]any
	invoice      map[string]any
	refunds      []any
	refundStatus string
	created      int
}

func newRefundWorld(t *testing.T) *refundWorld {
	t.Helper()
	w := &refundWorld{serviceWorld: newServiceWorld(t), refunds: []any{}, refundStatus: "succeeded"}
	w.stripe.current["items"].(map[string]any)["data"].([]any)[0].(map[string]any)["price"].(map[string]any)["recurring"] = map[string]any{
		"interval": "month",
	}
	w.warning = map[string]any{
		"id":             "issfr_expected",
		"charge":         "ch_expected",
		"payment_intent": "pi_expected",
		"actionable":     true,
	}
	w.charge = map[string]any{
		"id":                     "ch_expected",
		"customer":               "cus_expected",
		"payment_intent":         "pi_expected",
		"amount":                 1500,
		"amount_captured":        1500,
		"amount_refunded":        0,
		"captured":               true,
		"paid":                   true,
		"status":                 "succeeded",
		"currency":               "usd",
		"payment_method_details": map[string]any{"type": "card", "card": map[string]any{}},
	}
	w.payment = map[string]any{
		"id":          "inpay_expected",
		"invoice":     "in_expected",
		"status":      "paid",
		"currency":    "usd",
		"amount_paid": 1500,
		"payment":     map[string]any{"type": "payment_intent", "payment_intent": "pi_expected"},
	}
	w.invoice = map[string]any{"id": "in_expected", "customer": "cus_expected", "status": "paid", "currency": "usd",
		"parent": map[string]any{"type": "subscription_details", "subscription_details": map[string]any{
			"subscription": "sub_expected", "metadata": maps.Clone(w.stripe.current["metadata"].(map[string]string))}}}
	w.stripe.respond = func(method, path string, params map[string]string, idempotency string) (any, error) {
		switch path {
		case "/v1/radar/early_fraud_warnings/issfr_expected":
			return w.warning, nil
		case "/v1/charges/ch_expected":
			return w.charge, nil
		case "/v1/invoice_payments":
			return map[string]any{"data": []any{w.payment}}, nil
		case "/v1/invoices/in_expected":
			return w.invoice, nil
		case "/v1/subscriptions/sub_expected":
			return w.stripe.current, nil
		case "/v1/refunds":
			if method == "get" {
				return map[string]any{"data": w.refunds}, nil
			}
			stored, err := w.store.Snapshot()
			if err != nil {
				return nil, err
			}
			intent, ok := stored.FraudRefunds["ch_expected"]
			if !ok || intent.IdempotencyKey != idempotency || intent.Amount == 0 || intent.FirstAttemptAt.IsZero() {
				return nil, errors.New("refund submitted without a committed immutable intent")
			}
			w.created++
			if params["reason"] != "" {
				return nil, errors.New("refund policy must not modify Stripe block lists")
			}
			amount, parseErr := strconv.ParseInt(params["amount"], 10, 64)
			if parseErr != nil {
				return nil, parseErr
			}
			metadata := map[string]string{}
			for _, key := range []string{stripeInstallationMetadata, stripeProductMetadata, "anchor_organization_id", "anchor_refund_action_id"} {
				metadata[key] = params["metadata["+key+"]"]
			}
			refund := map[string]any{"id": "re_expected", "charge": "ch_expected", "amount": amount,
				"currency": "usd", "status": w.refundStatus, "metadata": metadata}
			w.refunds = append(w.refunds, refund)
			return refund, nil
		case "/v1/refunds/re_expected":
			return w.refunds[len(w.refunds)-1], nil
		default:
			return nil, fmt.Errorf("unexpected financial request %s %s", method, path)
		}
	}
	policy := FraudRefundPolicy{Enabled: true, Currency: FraudRefundCurrencyUsd, MaxAmount: 1500}
	_, err := w.service.UpdateSettings(t.Context(), UpdateSettingsRequest{FraudRefundPolicy: &policy})
	require.NoError(t, err)
	return w
}

func (w *refundWorld) send(t *testing.T, id, kind string, object map[string]any) {
	t.Helper()
	body, signature := signedEvent(t, w.config.WebhookSecret, id, kind, object)
	require.NoError(t, w.service.HandleWebhook(t.Context(), body, signature))
	require.NoError(t, w.service.ProcessPending(t.Context()))
}

func TestFraudRefundCommitsIntentBeforeRefundAndDeduplicatesCharge(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	w.send(t, "evt_warning", "radar.early_fraud_warning.created", w.warning)
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Len(t, state.FraudRefunds, 1)
	refund := state.FraudRefunds[0]
	require.Equal(t, FraudRefundSucceeded, refund.Status)
	require.Equal(t, int64(1500), refund.Amount)
	require.Equal(t, "re_expected", refund.RefundID)
	require.Equal(t, w.organization, refund.OrganizationID)
	require.Equal(t, "sub_expected", refund.SubscriptionID)
	require.Equal(t, "in_expected", refund.InvoiceID)
	require.Equal(t, 1, w.created)
	w.send(t, "evt_duplicate", "radar.early_fraud_warning.created", w.warning)
	secondWarning := maps.Clone(w.warning)
	secondWarning["id"] = "issfr_second"
	w.send(t, "evt_distinct_warning", fraudWarningCreated, secondWarning)
	w.send(t, "evt_updated", "radar.early_fraud_warning.updated", w.warning)
	require.Equal(t, 1, w.created)
	require.Empty(t, w.anchor.applications)
	stored, err := w.store.Snapshot()
	require.NoError(t, err)
	require.Equal(t, "sub_expected", stored.Organizations[w.organization].SubscriptionID)
}

func TestFraudRefundReceiptSavePreservesWebhookReceivedDuringProviderPost(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	respond := w.stripe.respond
	w.stripe.respond = func(method, path string, params map[string]string, idempotency string) (any, error) {
		if method == "post" && path == "/v1/refunds" {
			body, signature := signedEvent(
				t,
				w.config.WebhookSecret,
				"evt_during_refund",
				fraudWarningUpdated,
				w.warning,
			)
			if err := ReceiveWebhook(t.Context(), w.config, w.store, body, signature); err != nil {
				return nil, err
			}
		}
		return respond(method, path, params, idempotency)
	}
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	stored, err := w.store.Snapshot()
	require.NoError(t, err)
	require.Equal(t, FraudRefundSucceeded, stored.FraudRefunds["ch_expected"].Status)
	require.Equal(t, "re_expected", stored.FraudRefunds["ch_expected"].RefundID)
	received, exists := stored.Events["evt_during_refund"]
	require.True(t, exists)
	require.Equal(t, statusPending, received.Status)
	require.True(t, received.PolicyEnabled)
	require.Equal(t, "ch_expected", received.ChargeID)
	require.Equal(t, 1, w.created)
	require.NoError(t, w.service.ProcessPending(t.Context()))
	require.Equal(t, 1, w.created)
}

func TestFraudRefundScheduledMonitorRetryStopsAfterOriginalWindow(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		record := state.FraudRefunds["ch_expected"]
		record.CreatedAt = time.Now().UTC().Add(-refundReceiptMonitorWindow - time.Hour)
		state.FraudRefunds[record.ChargeID] = record
		state.Organizations = nil
		state.Events["evt_old_monitor"] = eventRecord{ID: "evt_old_monitor", Type: fraudReceiptMonitoring,
			ChargeID: record.ChargeID, ResourceID: record.RefundID, Status: statusError}
		return nil
	}))
	requests := len(w.stripe.requests)
	require.NoError(t, w.service.ProcessPending(t.Context()))
	require.Len(t, w.stripe.requests, requests)
	stored, err := w.store.Snapshot()
	require.NoError(t, err)
	require.Equal(t, "processed", stored.Events["evt_old_monitor"].Status)
}

func TestFraudRefundOwnershipAndEligibilityFailClosed(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		change func(*refundWorld)
		reason string
	}{
		"nonactionable":            {func(w *refundWorld) { w.warning["actionable"] = false }, "warning_not_actionable"},
		"live warning":             {func(w *refundWorld) { w.warning["livemode"] = true }, "ownership_unverified"},
		"live charge":              {func(w *refundWorld) { w.charge["livemode"] = true }, "ownership_unverified"},
		"wrong retrieved charge":   {func(w *refundWorld) { w.charge["id"] = "ch_foreign" }, "ownership_unverified"},
		"missing payment intent":   {func(w *refundWorld) { w.charge["payment_intent"] = nil }, "ownership_unverified"},
		"warning payment mismatch": {func(w *refundWorld) { w.warning["payment_intent"] = "pi_foreign" }, "ownership_unverified"},
		"foreign customer":         {func(w *refundWorld) { w.charge["customer"] = "cus_foreign" }, "ownership_unverified"},
		"uncaptured":               {func(w *refundWorld) { w.charge["captured"] = false }, "charge_not_eligible"},
		"partial capture":          {func(w *refundWorld) { w.charge["amount_captured"] = 1000 }, "charge_not_eligible"},
		"failed charge":            {func(w *refundWorld) { w.charge["status"] = "failed" }, "charge_not_eligible"},
		"unpaid":                   {func(w *refundWorld) { w.charge["paid"] = false }, "charge_not_eligible"},
		"noncard":                  {func(w *refundWorld) { w.charge["payment_method_details"] = map[string]any{"type": "link"} }, "charge_not_eligible"},
		"no card details":          {func(w *refundWorld) { w.charge["payment_method_details"] = map[string]any{"type": "card"} }, "charge_not_eligible"},
		"disputed charge":          {func(w *refundWorld) { w.charge["disputed"] = true }, "charge_disputed"},
		"different currency":       {func(w *refundWorld) { w.charge["currency"] = "cad" }, "currency_mismatch"},
		"original exceeds cap after partial refund": {func(w *refundWorld) {
			w.charge["amount"] = 3000
			w.charge["amount_captured"] = 3000
			w.charge["amount_refunded"] = 2000
		}, "amount_exceeds_limit"},
		"fully refunded": {func(w *refundWorld) { w.charge["amount_refunded"] = 1500 }, "no_refundable_balance"},
		"unowned payment": {func(w *refundWorld) {
			w.payment["payment"] = map[string]any{"type": "payment_intent", "payment_intent": "pi_foreign"}
		}, "ownership_unverified"},
		"live allocation":            {func(w *refundWorld) { w.payment["livemode"] = true }, "ownership_unverified"},
		"allocation amount mismatch": {func(w *refundWorld) { w.payment["amount_paid"] = 1000 }, "ownership_unverified"},
		"unpaid invoice":             {func(w *refundWorld) { w.invoice["status"] = "open" }, "ownership_unverified"},
		"live invoice":               {func(w *refundWorld) { w.invoice["livemode"] = true }, "ownership_unverified"},
		"invoice customer mismatch":  {func(w *refundWorld) { w.invoice["customer"] = "cus_foreign" }, "ownership_unverified"},
		"invoice no subscription":    {func(w *refundWorld) { w.invoice["parent"] = nil }, "ownership_unverified"},
		"foreign subscription":       {func(w *refundWorld) { w.stripe.current["id"] = "sub_foreign" }, "ownership_unverified"},
		"live subscription":          {func(w *refundWorld) { w.stripe.current["livemode"] = true }, "ownership_unverified"},
		"foreign subscription product": {func(w *refundWorld) {
			w.stripe.current["metadata"].(map[string]string)[stripeProductMetadata] = "prd_foreign"
		}, "ownership_unverified"},
		"foreign installation": {func(w *refundWorld) {
			w.stripe.current["metadata"].(map[string]string)[stripeInstallationMetadata] = "old_installation"
		}, "ownership_unverified"},
		"foreign invoice ownership snapshot": {func(w *refundWorld) {
			w.invoice["parent"].(map[string]any)["subscription_details"].(map[string]any)["metadata"].(map[string]string)[stripeProductMetadata] = "prd_foreign"
		}, "ownership_unverified"},
		"removed organization": {func(w *refundWorld) { w.anchor.snapshot.Organizations = nil }, "ownership_unverified"},
		"external pending refund": {func(w *refundWorld) {
			w.refunds = []any{map[string]any{"id": "re_external", "charge": "ch_expected", "status": "pending"}}
		}, "existing_refund_pending"},
		"external requires action": {func(w *refundWorld) {
			w.refunds = []any{map[string]any{"id": "re_external", "charge": "ch_expected", "status": "requires_action"}}
		}, "existing_refund_pending"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := newRefundWorld(t)
			test.change(w)
			w.send(t, "evt_warning", fraudWarningCreated, w.warning)
			state, err := w.service.State(t.Context())
			require.NoError(t, err)
			require.Len(t, state.FraudRefunds, 1)
			require.Equal(t, FraudRefundSkipped, state.FraudRefunds[0].Status)
			require.Equal(t, test.reason, state.FraudRefunds[0].Reason)
			require.Zero(t, w.created)
			require.Empty(t, w.anchor.applications)
		})
	}
}

func TestFraudRefundPolicyOffAtReceiptOrExecutionNeverBackfills(t *testing.T) {
	t.Parallel()
	for _, offAtReceipt := range []bool{true, false} {
		t.Run(strconv.FormatBool(offAtReceipt), func(t *testing.T) {
			t.Parallel()
			w := newRefundWorld(t)
			policy := FraudRefundPolicy{Currency: FraudRefundCurrencyUsd, MaxAmount: 1500}
			if offAtReceipt {
				_, err := w.service.UpdateSettings(t.Context(), UpdateSettingsRequest{FraudRefundPolicy: &policy})
				require.NoError(t, err)
			}
			body, sig := signedEvent(t, w.config.WebhookSecret, "evt_warning", fraudWarningCreated, w.warning)
			require.NoError(t, w.service.HandleWebhook(t.Context(), body, sig))
			policy.Enabled = offAtReceipt
			_, err := w.service.UpdateSettings(t.Context(), UpdateSettingsRequest{FraudRefundPolicy: &policy})
			require.NoError(t, err)
			require.NoError(t, w.service.ProcessPending(t.Context()))
			state, err := w.service.State(t.Context())
			require.NoError(t, err)
			require.Len(t, state.FraudRefunds, 1)
			require.Equal(t, "policy_disabled", state.FraudRefunds[0].Reason)
			policy.Enabled = true
			_, err = w.service.UpdateSettings(t.Context(), UpdateSettingsRequest{FraudRefundPolicy: &policy})
			require.NoError(t, err)
			w.send(t, "evt_updated", fraudWarningUpdated, w.warning)
			require.Zero(t, w.created)
			require.Empty(t, w.stripe.requests)
		})
	}
}

func TestFraudRefundPartialPriorRefundPaysOnlyRemainingBalance(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	w.charge["amount_refunded"] = 500
	w.refunds = []any{
		map[string]any{
			"id":       "re_prior",
			"charge":   "ch_expected",
			"status":   "succeeded",
			"amount":   500,
			"currency": "usd",
		},
	}
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Equal(t, int64(1000), state.FraudRefunds[0].Amount)
	require.Equal(t, FraudRefundSucceeded, state.FraudRefunds[0].Status)
}

func TestFraudRefundMonitorsPendingReceiptAfterPolicyDisabled(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"succeeded", "failed", "canceled", "requires_action"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			w := newRefundWorld(t)
			w.refundStatus = "pending"
			w.send(t, "evt_warning", fraudWarningCreated, w.warning)
			policy := FraudRefundPolicy{Currency: FraudRefundCurrencyUsd}
			_, err := w.service.UpdateSettings(t.Context(), UpdateSettingsRequest{FraudRefundPolicy: &policy})
			require.NoError(t, err)
			w.refunds[0].(map[string]any)["status"] = status
			w.send(t, "evt_refund", "refund.updated", w.refunds[0].(map[string]any))
			state, err := w.service.State(t.Context())
			require.NoError(t, err)
			require.Equal(t, FraudRefundStatus(status), state.FraudRefunds[0].Status)
			w.send(t, "evt_updated", fraudWarningUpdated, w.warning)
			require.Equal(t, 1, w.created)
			require.Empty(t, w.anchor.applications)
		})
	}
}

func (w *refundWorld) retry(t *testing.T) {
	t.Helper()
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		for id, event := range state.Events {
			event.NextAttempt = time.Time{}
			state.Events[id] = event
		}
		return nil
	}))
	service, err := NewService(w.config, w.stripe, w.anchor, w.store)
	require.NoError(t, err)
	w.service = service
	require.NoError(t, service.ProcessPending(t.Context()))
}

func TestFraudRefundLostAcceptedResponseRecoversReceiptWithoutSecondPost(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	respond := w.stripe.respond
	w.stripe.respond = func(method, path string, params map[string]string, key string) (any, error) {
		response, err := respond(method, path, params, key)
		if method == "post" && path == "/v1/refunds" {
			return nil, errors.New("ambiguous Stripe response sk_test_mustNeverLeak")
		}
		return response, err
	}
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Equal(t, FraudRefundProcessing, state.FraudRefunds[0].Status)
	require.NotContains(t, state.FraudRefunds[0].LastError, "sk_test_")
	policy := FraudRefundPolicy{Currency: FraudRefundCurrencyUsd}
	_, err = w.service.UpdateSettings(t.Context(), UpdateSettingsRequest{FraudRefundPolicy: &policy})
	require.NoError(t, err)
	w.retry(t)
	state, err = w.service.State(t.Context())
	require.NoError(t, err)
	require.Equal(t, FraudRefundSucceeded, state.FraudRefunds[0].Status)
	require.Equal(t, 1, w.created)
	for _, event := range state.Events {
		require.NotContains(t, event.LastError, "sk_test_")
	}
}

func TestFraudRefundUncertainRetryPinsPayloadAndStopsAfterReplayWindow(t *testing.T) {
	t.Parallel()
	for _, expired := range []bool{false, true} {
		t.Run(strconv.FormatBool(expired), func(t *testing.T) {
			t.Parallel()
			w := newRefundWorld(t)
			respond := w.stripe.respond
			first := true
			w.stripe.respond = func(method, path string, params map[string]string, key string) (any, error) {
				if method == "post" && path == "/v1/refunds" && first {
					first = false
					return nil, errors.New("network result unknown")
				}
				return respond(method, path, params, key)
			}
			w.send(t, "evt_warning", fraudWarningCreated, w.warning)
			stored, err := w.store.Snapshot()
			require.NoError(t, err)
			intent := stored.FraudRefunds["ch_expected"]
			if expired {
				require.NoError(t, w.store.Update(func(state *StoredState) error {
					record := state.FraudRefunds["ch_expected"]
					record.FirstAttemptAt = time.Now().Add(-refundReplayWindow)
					state.FraudRefunds["ch_expected"] = record
					return nil
				}))
			}
			w.retry(t)
			state, err := w.service.State(t.Context())
			require.NoError(t, err)
			if expired {
				require.Equal(t, FraudRefundReviewRequired, state.FraudRefunds[0].Status)
				require.Equal(t, "idempotency_window_expired", state.FraudRefunds[0].Reason)
				require.Zero(t, w.created)
			} else {
				require.Equal(t, FraudRefundSucceeded, state.FraudRefunds[0].Status)
				require.Equal(t, 1, w.created)
				posts := []stripeRequest{}
				for _, request := range w.stripe.requests {
					if request.method == "post" {
						posts = append(posts, request)
					}
				}
				require.Len(t, posts, 2)
				require.Equal(t, posts[0], posts[1])
				require.Equal(t, intent.IdempotencyKey, posts[1].idempotency)
			}
		})
	}
}

func TestFraudRefundRejectsAmbiguousInvoicePaymentPagination(t *testing.T) {
	t.Parallel()
	for name, response := range map[string]map[string]any{
		"multiple allocations": {"data": []any{map[string]any{"id": "inpay_1"}, map[string]any{"id": "inpay_2"}}},
		"null allocation":      {"data": []any{nil}},
		"empty continuation":   {"data": []any{}, "has_more": true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := newRefundWorld(t)
			respond := w.stripe.respond
			w.stripe.respond = func(method, path string, params map[string]string, key string) (any, error) {
				if path == "/v1/invoice_payments" {
					return response, nil
				}
				return respond(method, path, params, key)
			}
			w.send(t, "evt_warning", fraudWarningCreated, w.warning)
			require.Zero(t, w.created)
		})
	}
}

func TestFraudRefundWaitsForOwnedSubscriptionLinkWithoutChangingAccess(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		organization := state.Organizations[w.organization]
		organization.SubscriptionID = ""
		state.Organizations[w.organization] = organization
		return nil
	}))
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Empty(t, state.FraudRefunds)
	require.Equal(t, statusError, state.Events[0].Status)
	require.Zero(t, w.created)
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		organization := state.Organizations[w.organization]
		organization.SubscriptionID = "sub_expected"
		state.Organizations[w.organization] = organization
		return nil
	}))
	w.retry(t)
	state, err = w.service.State(t.Context())
	require.NoError(t, err)
	require.Equal(t, FraudRefundSucceeded, state.FraudRefunds[0].Status)
	require.Equal(t, 1, w.created)
	require.Empty(t, w.anchor.applications)
}

func TestFraudRefundOwnedHistoricalInvoiceKeepsCurrentSubscription(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		organization := state.Organizations[w.organization]
		organization.SubscriptionID = "sub_new_current"
		state.Organizations[w.organization] = organization
		return nil
	}))
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Equal(t, FraudRefundSucceeded, state.FraudRefunds[0].Status)
	stored, err := w.store.Snapshot()
	require.NoError(t, err)
	require.Equal(t, "sub_new_current", stored.Organizations[w.organization].SubscriptionID)
	require.Empty(t, w.anchor.applications)
}

func TestFraudRefundFreshnessAfterCommittedIntentStopsBeforePost(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	respond := w.stripe.respond
	w.stripe.respond = func(method, path string, params map[string]string, key string) (any, error) {
		if path == "/v1/radar/early_fraud_warnings/issfr_expected" {
			stored, err := w.store.Snapshot()
			if err != nil {
				return nil, err
			}
			if _, exists := stored.FraudRefunds["ch_expected"]; exists {
				w.warning["actionable"] = false
			}
		}
		return respond(method, path, params, key)
	}
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Len(t, state.FraudRefunds, 1)
	require.Equal(t, FraudRefundSkipped, state.FraudRefunds[0].Status)
	require.Equal(t, "warning_not_actionable", state.FraudRefunds[0].Reason)
	require.Zero(t, w.created)
	w.warning["actionable"] = true
	w.send(t, "evt_updated", fraudWarningUpdated, w.warning)
	require.Zero(t, w.created)
}

type faultRefundStore struct {
	StateStore
	reject func(StoredState) bool
}

func (s faultRefundStore) Update(change func(*StoredState) error) error {
	return s.StateStore.Update(func(state *StoredState) error {
		if err := change(state); err != nil {
			return err
		}
		if s.reject(*state) {
			return errors.New("simulated failed financial persistence")
		}
		return nil
	})
}

func TestFraudRefundCrashAfterAcceptedAPIRecoversMissingLocalReceipt(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	w.service.store = faultRefundStore{StateStore: w.store, reject: func(state StoredState) bool {
		return state.FraudRefunds["ch_expected"].Status == FraudRefundSucceeded
	}}
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	stored, err := w.store.Snapshot()
	require.NoError(t, err)
	require.Equal(t, FraudRefundProcessing, stored.FraudRefunds["ch_expected"].Status)
	require.Empty(t, stored.FraudRefunds["ch_expected"].RefundID)
	w.retry(t)
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Equal(t, FraudRefundSucceeded, state.FraudRefunds[0].Status)
	require.Equal(t, 1, w.created)
}

func TestFraudRefundCannotPostWhenIntentPersistenceFails(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	w.service.store = faultRefundStore{StateStore: w.store, reject: func(state StoredState) bool {
		return len(state.FraudRefunds) > 0
	}}
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	require.Zero(t, w.created)
	stored, err := w.store.Snapshot()
	require.NoError(t, err)
	require.Empty(t, stored.FraudRefunds)
}

func TestFraudRefundLaterInvoicePageFailurePreventsFinancialMutation(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	respond := w.stripe.respond
	w.stripe.respond = func(method, path string, params map[string]string, key string) (any, error) {
		if path == "/v1/invoice_payments" {
			if params["starting_after"] != "" {
				return nil, errors.New("later page unavailable")
			}
			return map[string]any{"data": []any{w.payment}, "has_more": true}, nil
		}
		return respond(method, path, params, key)
	}
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	require.Zero(t, w.created)
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Empty(t, state.FraudRefunds)
	require.Equal(t, statusError, state.Events[0].Status)
}

func TestFraudRefundBoundsFullFinancialPagination(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/v1/invoice_payments", "/v1/refunds"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			w := newRefundWorld(t)
			respond := w.stripe.respond
			w.stripe.respond = func(method, target string, params map[string]string, key string) (any, error) {
				if method == "get" && target == path {
					data := make([]any, maximumFinancialResources+1)
					for index := range data {
						data[index] = map[string]any{"id": fmt.Sprintf("object_%d", index), "charge": "ch_expected"}
					}
					return map[string]any{"data": data}, nil
				}
				return respond(method, target, params, key)
			}
			w.send(t, "evt_warning", fraudWarningCreated, w.warning)
			require.Zero(t, w.created)
			state, err := w.service.State(t.Context())
			require.NoError(t, err)
			require.Empty(t, state.FraudRefunds)
			require.Equal(t, statusError, state.Events[0].Status)
		})
	}
}

func TestFraudRefundUnknownProviderStatusRemainsReviewWithoutReplay(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	w.refundStatus = "future_status"
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Equal(t, FraudRefundReviewRequired, state.FraudRefunds[0].Status)
	require.Equal(t, "refund_status_unknown", state.FraudRefunds[0].Reason)
	w.send(t, "evt_updated", fraudWarningUpdated, w.warning)
	require.Equal(t, 1, w.created)
}

type refundIdentityGateway struct {
	StripeGateway
	identity Account
}

func (s refundIdentityGateway) Account(context.Context) (Account, error) { return s.identity, nil }

func TestFraudRefundRejectsForeignAccountAndLiveIdentityBeforeResources(t *testing.T) {
	t.Parallel()
	for name, identity := range map[string]Account{
		"foreign account": {ID: "acct_foreign", Mode: Sandbox},
		"live identity":   {ID: "acct_expected", Mode: "live"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := newRefundWorld(t)
			w.service.stripe = refundIdentityGateway{StripeGateway: w.stripe, identity: identity}
			w.send(t, "evt_warning", fraudWarningCreated, w.warning)
			require.Empty(t, w.stripe.requests)
			require.Zero(t, w.created)
			stored, err := w.store.Snapshot()
			require.NoError(t, err)
			require.Equal(t, "ownership_unverified", stored.FraudRefunds["ch_expected"].Reason)
		})
	}
}

func TestFraudRefundExpiryStillRecoversAcceptedRemoteReceipt(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	w.service.store = faultRefundStore{StateStore: w.store, reject: func(state StoredState) bool {
		return state.FraudRefunds["ch_expected"].Status == FraudRefundSucceeded
	}}
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		record := state.FraudRefunds["ch_expected"]
		record.FirstAttemptAt = time.Now().Add(-refundReplayWindow)
		state.FraudRefunds["ch_expected"] = record
		return nil
	}))
	w.retry(t)
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Equal(t, FraudRefundSucceeded, state.FraudRefunds[0].Status)
	require.Equal(t, 1, w.created)
}

func TestFraudRefundDisputeEventNeverRequestsRefund(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	w.send(
		t,
		"evt_dispute",
		"charge.dispute.created",
		map[string]any{"id": "dp_expected", "charge": "ch_expected", "status": "needs_response"},
	)
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Empty(t, state.FraudRefunds)
	require.Zero(t, w.created)
	require.Empty(t, w.stripe.requests)
}

func TestFraudRefundProviderEventsOnlyMonitorExistingIntent(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"charge.dispute.created", "charge.dispute.updated", "refund.created", "refund.updated", "refund.failed"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			w := newRefundWorld(t)
			respond := w.stripe.respond
			first := true
			w.stripe.respond = func(method, path string, params map[string]string, key string) (any, error) {
				if method == "post" && path == "/v1/refunds" && first {
					first = false
					return nil, errors.New("refund request result unknown")
				}
				return respond(method, path, params, key)
			}
			w.send(t, "evt_warning", fraudWarningCreated, w.warning)
			w.send(t, "evt_monitor", kind, map[string]any{"id": "re_or_dispute", "charge": "ch_expected"})
			require.Zero(t, w.created, "provider status events must never create or replay a refund")
		})
	}
}

func TestFraudRefundExplicitFailureRefreshesPriorSuccessfulReceipt(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	w.refunds[0].(map[string]any)["status"] = "failed"
	w.send(t, "evt_failure", "refund.failed", w.refunds[0].(map[string]any))
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Equal(t, FraudRefundFailed, state.FraudRefunds[0].Status)
	require.Equal(t, "refund_failed", state.FraudRefunds[0].Reason)
	require.Equal(t, 1, w.created)
}

func TestFraudRefundScheduledTerminalReceiptMonitorRepairsMissedFailureWithoutPost(t *testing.T) {
	t.Parallel()
	w := newRefundWorld(t)
	w.send(t, "evt_warning", fraudWarningCreated, w.warning)
	w.refunds[0].(map[string]any)["status"] = "failed"
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		state.Organizations = map[string]organizationRecord{}
		state.Settings.FraudRefundPolicy.Enabled = false
		return nil
	}))
	require.NoError(t, w.service.QueueReconciliation())
	require.NoError(t, w.service.ProcessPending(t.Context()))
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	require.Equal(t, FraudRefundFailed, state.FraudRefunds[0].Status)
	require.Equal(t, 1, w.created)
}

func TestFraudRefundTerminalReceiptMonitoringIsBoundedAndRecent(t *testing.T) {
	t.Parallel()
	state := NewStoredState("acct_expected", "prd_expected")
	now := time.Now().UTC()
	for index := range maximumReceiptMonitors + 1 {
		id := fmt.Sprintf("action_%d", index)
		state.FraudRefunds[id] = fraudRefundRecord{ID: id, ChargeID: id, RefundID: "re_" + id, IdempotencyKey: id,
			Status: FraudRefundSucceeded, CreatedAt: now, UpdatedAt: now.Add(time.Duration(index) * time.Second)}
	}
	state.FraudRefunds["old"] = fraudRefundRecord{ID: "old", ChargeID: "old", RefundID: "re_old", IdempotencyKey: "old",
		Status: FraudRefundSucceeded, CreatedAt: now.Add(-refundReceiptMonitorWindow - time.Hour)}
	queueTerminalRefundMonitoring(&state)
	require.Len(t, state.Events, maximumReceiptMonitors)
	require.NotContains(t, state.Events, "fraud-monitor-old")
	for _, event := range state.Events {
		require.Equal(t, fraudReceiptMonitoring, event.Type)
	}
}

func TestHasUnresolvedRefundsFailsClosed(t *testing.T) {
	t.Parallel()
	for _, status := range []FraudRefundStatus{FraudRefundSkipped, FraudRefundSucceeded, FraudRefundFailed, FraudRefundCanceled,
		FraudRefundProcessing, FraudRefundPending, FraudRefundRequiresAction, FraudRefundReviewRequired, "unknown", ""} {
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			state := StoredState{
				FraudRefunds: map[string]fraudRefundRecord{"ch_expected": {FraudRefund: FraudRefund{Status: status}}},
			}
			expected := status != FraudRefundSkipped && status != FraudRefundSucceeded && status != FraudRefundFailed &&
				status != FraudRefundCanceled
			require.Equal(t, expected, HasUnresolvedRefunds(state))
		})
	}
}

func TestFraudRefundPolicyDefaultsAndPartialUpdates(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	ctx := context.Background()
	state, err := w.service.State(ctx)
	require.NoError(t, err)
	require.Equal(t, FraudRefundPolicy{Currency: FraudRefundCurrencyUsd}, state.Settings.FraudRefundPolicy)
	require.NotNil(t, state.FraudRefunds)

	policy := FraudRefundPolicy{Enabled: true, Currency: FraudRefundCurrencyCad, MaxAmount: 1500}
	settings, err := w.service.UpdateSettings(ctx, UpdateSettingsRequest{FraudRefundPolicy: &policy})
	require.NoError(t, err)
	require.Equal(t, w.freeTemplate, settings.FallbackTemplateID)
	require.Equal(t, policy, settings.FraudRefundPolicy)
	settings, err = w.service.UpdateSettings(ctx, UpdateSettingsRequest{FallbackTemplateID: &w.paidTemplate})
	require.NoError(t, err)
	require.Equal(t, w.paidTemplate, settings.FallbackTemplateID)
	require.Equal(t, policy, settings.FraudRefundPolicy)
	require.Empty(t, w.stripe.requests)
}

func TestFraudRefundPolicyRejectsInvalidUpdates(t *testing.T) {
	t.Parallel()
	for name, request := range map[string]UpdateSettingsRequest{
		"empty":                {},
		"enabled zero cap":     {FraudRefundPolicy: &FraudRefundPolicy{Enabled: true, Currency: FraudRefundCurrencyUsd}},
		"unsupported currency": {FraudRefundPolicy: &FraudRefundPolicy{Currency: "gbp"}},
		"negative cap":         {FraudRefundPolicy: &FraudRefundPolicy{Currency: FraudRefundCurrencyUsd, MaxAmount: -1}},
		"oversized cap":        {FraudRefundPolicy: &FraudRefundPolicy{Currency: FraudRefundCurrencyUsd, MaxAmount: 100000000}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := newServiceWorld(t)
			_, err := w.service.UpdateSettings(context.Background(), request)
			require.ErrorIs(t, err, ErrInput)
			require.Empty(t, w.stripe.requests)
		})
	}
}

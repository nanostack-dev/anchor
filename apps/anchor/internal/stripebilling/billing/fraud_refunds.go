package billing

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/segmentio/ksuid"
	"github.com/stripe/stripe-go/v87"
)

const (
	fraudWarningCreated          = "radar.early_fraud_warning.created"
	fraudWarningUpdated          = "radar.early_fraud_warning.updated"
	fraudReconciliation          = "fraud.refund.reconciliation"
	fraudReceiptMonitoring       = "fraud.refund.monitor"
	refundActionMetadata         = "anchor_refund_action_id"
	maximumFinancialResources    = 1000
	refundReplayWindow           = 23 * time.Hour
	maximumOwnershipLinkAttempts = 8
	refundReceiptMonitorWindow   = 31 * 24 * time.Hour
	maximumReceiptMonitors       = 10
)

var errRefundSubscriptionLinkPending = errors.New("owned subscription is awaiting Anchor reconciliation")

func financialEvent(kind string) bool {
	return kind == fraudWarningCreated || kind == fraudWarningUpdated || kind == fraudReconciliation ||
		kind == fraudReceiptMonitoring ||
		kind == "refund.created" ||
		kind == "refund.updated" ||
		kind == "refund.failed" ||
		strings.HasPrefix(kind, "charge.dispute.")
}

func (s *Service) processFinancialEvent(ctx context.Context, event eventRecord) error {
	stored, err := s.store.Snapshot()
	if err != nil {
		return err
	}
	if strings.HasPrefix(event.Type, "charge.dispute.") {
		if record, exists := stored.FraudRefunds[event.ChargeID]; exists && record.IdempotencyKey != "" {
			return s.reconcileFraudRefund(ctx, record, false)
		}
		return nil
	}
	if event.Type == fraudReconciliation || event.Type == fraudReceiptMonitoring ||
		strings.HasPrefix(event.Type, "refund.") {
		key := event.ChargeID
		if key == "" {
			key = functional.Slice(slices.Collect(maps.Keys(stored.FraudRefunds))).FindFirst(func(id string) bool {
				return stored.FraudRefunds[id].RefundID == event.ResourceID
			}).OrElse("")
		}
		if record, exists := stored.FraudRefunds[key]; exists && record.IdempotencyKey != "" {
			if event.Type == fraudReceiptMonitoring &&
				record.CreatedAt.Before(time.Now().UTC().Add(-refundReceiptMonitorWindow)) {
				return nil
			}
			return s.reconcileFraudRefund(ctx, record, event.Type == fraudReconciliation)
		}
		return nil
	}
	if record, exists := stored.FraudRefunds[event.ChargeID]; exists {
		if record.IdempotencyKey != "" {
			return s.recoverFraudRefund(ctx, record)
		}
		return nil
	}
	if event.Type != fraudWarningCreated {
		return nil
	}
	return s.evaluateFraudWarning(ctx, event, stored)
}

func queueTerminalRefundMonitoring(state *StoredState) {
	now := time.Now().UTC()
	records := functional.Slice(slices.Collect(maps.Values(state.FraudRefunds))).
		Filter(func(record fraudRefundRecord) bool {
			return record.RefundID != "" && record.IdempotencyKey != "" &&
				(record.Status == FraudRefundSucceeded || record.Status == FraudRefundFailed || record.Status == FraudRefundCanceled) &&
				!record.CreatedAt.IsZero() && !record.CreatedAt.Before(now.Add(-refundReceiptMonitorWindow))
		}).
		SortedBy(func(record fraudRefundRecord) int64 { return record.UpdatedAt.UnixNano() })
	if len(records) > maximumReceiptMonitors {
		records = records[:maximumReceiptMonitors]
	}
	for _, record := range records {
		id := "fraud-monitor-" + record.ID
		if event, exists := state.Events[id]; exists && (event.Status == statusPending || event.Status == statusError) {
			continue
		}
		state.Events[id] = eventRecord{
			ID:             id,
			Type:           fraudReceiptMonitoring,
			OrganizationID: record.OrganizationID,
			Status:         statusPending,
			ReceivedAt:     now,
			ChargeID:       record.ChargeID,
			ResourceID:     record.RefundID,
			NextAttempt:    now,
		}
	}
}

func (s *Service) evaluateFraudWarning(ctx context.Context, event eventRecord, stored StoredState) error {
	if !event.PolicyEnabled || !normalizedSettings(stored.Settings).FraudRefundPolicy.Enabled {
		return s.skipFraudWarning(event, "policy_disabled")
	}
	account, err := s.stripe.Account(ctx)
	if err != nil {
		return err
	}
	if account.ID != s.config.ExpectedAccountID || account.Mode != Sandbox {
		return s.skipFraudWarning(event, "ownership_unverified")
	}
	warning, err := s.stripe.Client().V1RadarEarlyFraudWarnings.Retrieve(ctx, event.ResourceID, nil)
	if err != nil {
		return err
	}
	if warning.ID != event.ResourceID || warning.Livemode || warning.Charge == nil ||
		warning.Charge.ID == "" || warning.Charge.ID != event.ChargeID {
		return s.skipFraudWarning(event, "ownership_unverified")
	}
	if !warning.Actionable {
		return s.skipFraudWarning(event, "warning_not_actionable")
	}
	charge, err := s.stripe.Client().V1Charges.Retrieve(ctx, warning.Charge.ID, nil)
	if err != nil {
		return err
	}
	if charge.ID != warning.Charge.ID {
		return s.skipFraudWarning(event, "ownership_unverified")
	}
	policy := normalizedSettings(stored.Settings).FraudRefundPolicy
	if reason := chargeEligibility(charge, policy); reason != "" {
		return s.skipFraudWarning(event, reason)
	}
	if warning.PaymentIntent != nil &&
		(charge.PaymentIntent == nil || warning.PaymentIntent.ID != charge.PaymentIntent.ID) {
		return s.skipFraudWarning(event, "ownership_unverified")
	}
	owned, err := s.ownedRefundCharge(ctx, charge, stored)
	if errors.Is(err, errRefundSubscriptionLinkPending) && event.Attempts >= maximumOwnershipLinkAttempts {
		return s.skipFraudWarning(event, "ownership_unverified")
	}
	if err != nil {
		return err
	}
	if owned == nil {
		return s.skipFraudWarning(event, "ownership_unverified")
	}
	refunds, err := s.chargeRefunds(ctx, charge.ID)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(refunds, func(refund *stripe.Refund) bool {
		return refund.Status == stripe.RefundStatusPending || refund.Status == stripe.RefundStatusRequiresAction
	}) {
		return s.skipFraudWarning(event, "existing_refund_pending")
	}
	now := time.Now().UTC()
	owned.FraudRefund = FraudRefund{ID: ksuid.New().String(), WarningID: warning.ID, ChargeID: charge.ID,
		InvoiceID: owned.InvoiceID, SubscriptionID: owned.SubscriptionID, OrganizationID: owned.OrganizationID,
		Amount: charge.AmountCaptured - charge.AmountRefunded, Currency: string(charge.Currency),
		Status: FraudRefundProcessing, Reason: "refund_requested", CreatedAt: now, UpdatedAt: now}
	owned.IdempotencyKey = stored.InstallationID + "-fraud-refund-" + charge.ID
	owned.InstallationID, owned.ProductID = stored.InstallationID, stored.ProductID
	if err = s.saveFraudRefund(*owned); err != nil {
		return err
	}
	return s.issueFraudRefund(ctx, *owned)
}

func chargeEligibility(charge *stripe.Charge, policy FraudRefundPolicy) string {
	if charge == nil || charge.ID == "" || charge.Livemode || charge.Customer == nil || charge.Customer.ID == "" ||
		charge.PaymentIntent == nil || charge.PaymentIntent.ID == "" {
		return "ownership_unverified"
	}
	if charge.Disputed {
		return "charge_disputed"
	}
	if !charge.Captured || !charge.Paid || charge.Status != stripe.ChargeStatusSucceeded ||
		charge.PaymentMethodDetails == nil || charge.PaymentMethodDetails.Type != stripe.ChargePaymentMethodDetailsTypeCard ||
		charge.PaymentMethodDetails.Card == nil || charge.Amount <= 0 || charge.AmountCaptured != charge.Amount ||
		charge.AmountRefunded < 0 || charge.AmountRefunded > charge.AmountCaptured {
		return "charge_not_eligible"
	}
	if string(charge.Currency) != string(policy.Currency) {
		return "currency_mismatch"
	}
	if charge.Amount > policy.MaxAmount {
		return "amount_exceeds_limit"
	}
	if charge.AmountCaptured-charge.AmountRefunded <= 0 {
		return "no_refundable_balance"
	}
	return ""
}

func (s *Service) ownedRefundCharge(
	ctx context.Context,
	charge *stripe.Charge,
	stored StoredState,
) (*fraudRefundRecord, error) {
	payments, err := s.invoicePayments(ctx, charge.PaymentIntent.ID)
	if err != nil {
		return nil, err
	}
	if len(payments) != 1 {
		return nil, nil
	}
	payment := payments[0]
	if !validRefundPayment(payment, charge) {
		return nil, nil
	}
	invoice, err := s.stripe.Client().V1Invoices.Retrieve(ctx, payment.Invoice.ID, nil)
	if err != nil {
		return nil, err
	}
	if !validRefundInvoice(invoice, payment.Invoice.ID, charge) {
		return nil, nil
	}
	details := invoice.Parent.SubscriptionDetails
	subscription, err := s.stripe.Client().V1Subscriptions.Retrieve(ctx, details.Subscription.ID, nil)
	if err != nil {
		return nil, err
	}
	organizationID := subscription.Metadata[stripeOrganizationMetadata]
	organization, exists := stored.Organizations[organizationID]
	if subscription.ID != details.Subscription.ID || subscription.Livemode || subscription.Customer == nil ||
		subscription.Customer.ID != charge.Customer.ID || !exists || organization.ID != organizationID ||
		organization.CustomerID != charge.Customer.ID ||
		!refundOwnershipMetadata(subscription.Metadata, stored.InstallationID, stored.ProductID, organizationID) ||
		!refundOwnershipMetadata(details.Metadata, stored.InstallationID, stored.ProductID, organizationID) {
		return nil, nil
	}
	if !validSubscriptionItem(subscription) {
		return nil, nil
	}
	itemPrice := subscriptionItems(subscription)[0].Price
	price := functional.Slice(slices.Collect(maps.Values(stored.Prices))).FindFirst(func(price Price) bool {
		return price.StripePriceID == itemPrice.ID
	})
	if price.IsAbsent() || itemPrice.Livemode || itemPrice.Recurring == nil ||
		string(itemPrice.Recurring.Interval) != string(price.Value().Interval) {
		return nil, nil
	}
	if _, err = s.organizationSource(ctx, organizationID); errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if organization.SubscriptionID == "" {
		return nil, errRefundSubscriptionLinkPending
	}
	return &fraudRefundRecord{InvoiceID: invoice.ID, SubscriptionID: subscription.ID,
		OrganizationID: organizationID, CustomerID: charge.Customer.ID}, nil
}

func validRefundPayment(payment *stripe.InvoicePayment, charge *stripe.Charge) bool {
	return payment != nil && !payment.Livemode && payment.Status == "paid" && payment.Invoice != nil &&
		payment.Invoice.ID != "" &&
		payment.Payment != nil &&
		payment.Payment.Type == stripe.InvoicePaymentPaymentTypePaymentIntent &&
		payment.Payment.PaymentIntent != nil &&
		payment.Payment.PaymentIntent.ID == charge.PaymentIntent.ID &&
		payment.Currency == charge.Currency &&
		payment.AmountPaid == charge.AmountCaptured
}

func validRefundInvoice(invoice *stripe.Invoice, invoiceID string, charge *stripe.Charge) bool {
	return invoice != nil && invoice.ID == invoiceID && !invoice.Livemode &&
		invoice.Status == stripe.InvoiceStatusPaid &&
		invoice.Customer != nil &&
		invoice.Customer.ID == charge.Customer.ID &&
		invoice.Currency == charge.Currency &&
		invoice.Parent != nil &&
		invoice.Parent.Type == stripe.InvoiceParentTypeSubscriptionDetails &&
		invoice.Parent.SubscriptionDetails != nil &&
		invoice.Parent.SubscriptionDetails.Subscription != nil &&
		invoice.Parent.SubscriptionDetails.Subscription.ID != ""
}

func refundOwnershipMetadata(metadata map[string]string, installationID, productID, organizationID string) bool {
	return installationID != "" && productID != "" && organizationID != "" &&
		metadata[stripeInstallationMetadata] == installationID && metadata[stripeProductMetadata] == productID &&
		metadata[stripeOrganizationMetadata] == organizationID
}

func (s *Service) invoicePayments(ctx context.Context, paymentIntentID string) ([]*stripe.InvoicePayment, error) {
	params := &stripe.InvoicePaymentListParams{Limit: new(int64(stripeSubscriptionPageSize)), Status: new("paid"),
		Payment: &stripe.InvoicePaymentListPaymentParams{Type: new("payment_intent"), PaymentIntent: &paymentIntentID}}
	result := []*stripe.InvoicePayment{}
	for {
		page := s.stripe.Client().V1InvoicePayments.List(ctx, params)
		data := page.Data()
		if err := page.Err(); err != nil {
			return nil, err
		}
		if len(data) == 0 && page.Meta().HasMore ||
			slices.ContainsFunc(data, func(item *stripe.InvoicePayment) bool { return item == nil || item.ID == "" }) {
			return nil, errors.New("stripe returned incomplete invoice payment ownership")
		}
		result = append(result, data...)
		if len(result) > maximumFinancialResources {
			return nil, errors.New("stripe invoice payment ownership exceeds the safe pagination limit")
		}
		if !page.Meta().HasMore {
			return result, nil
		}
		params.StartingAfter = &data[len(data)-1].ID
	}
}

func (s *Service) chargeRefunds(ctx context.Context, chargeID string) ([]*stripe.Refund, error) {
	params := &stripe.RefundListParams{Charge: &chargeID, Limit: new(int64(stripeSubscriptionPageSize))}
	result := []*stripe.Refund{}
	for {
		page := s.stripe.Client().V1Refunds.List(ctx, params)
		data := page.Data()
		if err := page.Err(); err != nil {
			return nil, err
		}
		if len(data) == 0 && page.Meta().HasMore || slices.ContainsFunc(data, func(item *stripe.Refund) bool {
			return item == nil || item.ID == "" || item.Charge == nil || item.Charge.ID != chargeID
		}) {
			return nil, errors.New("stripe returned incomplete charge refund history")
		}
		result = append(result, data...)
		if len(result) > maximumFinancialResources {
			return nil, errors.New("stripe charge refund history exceeds the safe pagination limit")
		}
		if !page.Meta().HasMore {
			return result, nil
		}
		params.StartingAfter = &data[len(data)-1].ID
	}
}

func (s *Service) skipFraudWarning(event eventRecord, reason string) error {
	key := event.ChargeID
	if key == "" {
		key = "warning:" + event.ResourceID
	}
	return s.store.Update(func(state *StoredState) error {
		if state.FraudRefunds == nil {
			state.FraudRefunds = map[string]fraudRefundRecord{}
		}
		if _, exists := state.FraudRefunds[key]; exists {
			return nil
		}
		now := time.Now().UTC()
		state.FraudRefunds[key] = fraudRefundRecord{
			ID: ksuid.New().String(), WarningID: event.ResourceID,
			ChargeID: event.ChargeID, Status: FraudRefundSkipped, Reason: reason, CreatedAt: now, UpdatedAt: now,
		}
		return nil
	})
}

func (s *Service) saveFraudRefund(record fraudRefundRecord) error {
	return s.store.Update(func(state *StoredState) error {
		if state.FraudRefunds == nil {
			state.FraudRefunds = map[string]fraudRefundRecord{}
		}
		state.FraudRefunds[record.ChargeID] = record
		return nil
	})
}

func (s *Service) issueFraudRefund(ctx context.Context, record fraudRefundRecord) error {
	stored, err := s.store.Snapshot()
	if err != nil {
		return err
	}
	if !normalizedSettings(stored.Settings).FraudRefundPolicy.Enabled {
		return s.stopFraudRefund(record, "policy_disabled")
	}
	warning, err := s.stripe.Client().V1RadarEarlyFraudWarnings.Retrieve(ctx, record.WarningID, nil)
	if err != nil {
		return err
	}
	if warning.ID != record.WarningID || warning.Livemode || !warning.Actionable || warning.Charge == nil ||
		warning.Charge.ID != record.ChargeID {
		return s.stopFraudRefund(record, "warning_not_actionable")
	}
	charge, err := s.stripe.Client().V1Charges.Retrieve(ctx, record.ChargeID, nil)
	if err != nil {
		return err
	}
	if charge.ID != record.ChargeID ||
		chargeEligibility(charge, normalizedSettings(stored.Settings).FraudRefundPolicy) != "" ||
		charge.AmountCaptured-charge.AmountRefunded != record.Amount ||
		charge.Customer.ID != record.CustomerID {
		return s.stopFraudRefund(record, "charge_changed")
	}
	if record.FirstAttemptAt.IsZero() {
		record.FirstAttemptAt = time.Now().UTC()
		if err = s.saveFraudRefund(record); err != nil {
			return err
		}
	}
	refund, err := s.stripe.Client().V1Refunds.Create(ctx, &stripe.RefundCreateParams{
		IdempotencyKey: &record.IdempotencyKey,
		Charge:         &record.ChargeID,
		Amount:         &record.Amount,
		Metadata: map[string]string{
			stripeInstallationMetadata: record.InstallationID,
			stripeProductMetadata:      record.ProductID,
			stripeOrganizationMetadata: record.OrganizationID,
			refundActionMetadata:       record.ID,
		},
	})
	if err != nil {
		record.LastError = "Stripe has not confirmed this refund; Anchor will reconcile the saved intent."
		record.UpdatedAt = time.Now().UTC()
		return errors.Join(err, s.saveFraudRefund(record))
	}
	return s.saveRefundReceipt(record, refund)
}

func (s *Service) stopFraudRefund(record fraudRefundRecord, reason string) error {
	if !record.FirstAttemptAt.IsZero() {
		return s.reviewFraudRefund(record, reason)
	}
	record.Status, record.Reason, record.LastError, record.UpdatedAt = FraudRefundSkipped, reason, "", time.Now().UTC()
	return s.saveFraudRefund(record)
}

func (s *Service) saveRefundReceipt(record fraudRefundRecord, refund *stripe.Refund) error {
	if refund == nil || refund.ID == "" || refund.Charge == nil || refund.Charge.ID != record.ChargeID ||
		refund.Amount != record.Amount || string(refund.Currency) != record.Currency ||
		refund.Metadata[refundActionMetadata] != record.ID ||
		!refundOwnershipMetadata(refund.Metadata, record.InstallationID, record.ProductID, record.OrganizationID) {
		return s.reviewFraudRefund(record, "refund_receipt_unverified")
	}
	status := FraudRefundStatus(refund.Status)
	if status != FraudRefundPending && status != FraudRefundRequiresAction && status != FraudRefundSucceeded &&
		status != FraudRefundFailed && status != FraudRefundCanceled {
		return s.reviewFraudRefund(record, "refund_status_unknown")
	}
	record.RefundID, record.Status, record.Reason = refund.ID, status, "refund_status_updated"
	if status == FraudRefundFailed {
		record.Reason = "refund_failed"
	}
	record.LastError, record.UpdatedAt = "", time.Now().UTC()
	return s.saveFraudRefund(record)
}

func (s *Service) reviewFraudRefund(record fraudRefundRecord, reason string) error {
	record.Status, record.Reason, record.UpdatedAt = FraudRefundReviewRequired, reason, time.Now().UTC()
	record.LastError = "The refund needs manual review. Anchor will not issue another refund automatically."
	return s.saveFraudRefund(record)
}

func (s *Service) recoverFraudRefund(ctx context.Context, record fraudRefundRecord) error {
	return s.reconcileFraudRefund(ctx, record, true)
}

func (s *Service) reconcileFraudRefund(ctx context.Context, record fraudRefundRecord, allowRetry bool) error {
	if record.Status == FraudRefundSkipped {
		return nil
	}
	account, err := s.stripe.Account(ctx)
	if err != nil {
		return err
	}
	stored, err := s.store.Snapshot()
	if err != nil {
		return err
	}
	if account.ID != s.config.ExpectedAccountID || account.Mode != Sandbox ||
		record.InstallationID != stored.InstallationID ||
		record.ProductID != stored.ProductID {
		return s.reviewFraudRefund(record, "ownership_unverified")
	}
	if record.RefundID != "" {
		if allowRetry && (record.Status == FraudRefundSucceeded || record.Status == FraudRefundFailed ||
			record.Status == FraudRefundCanceled) {
			return nil
		}
		refund, retrieveErr := s.stripe.Client().V1Refunds.Retrieve(ctx, record.RefundID, nil)
		if retrieveErr != nil {
			return retrieveErr
		}
		return s.saveRefundReceipt(record, refund)
	}
	refunds, err := s.chargeRefunds(ctx, record.ChargeID)
	if err != nil {
		return err
	}
	matching := functional.Slice(refunds).
		Filter(func(refund *stripe.Refund) bool { return refund.Metadata[refundActionMetadata] == record.ID })
	if len(matching) == 1 {
		return s.saveRefundReceipt(record, matching[0])
	}
	if len(matching) > 1 {
		return s.reviewFraudRefund(record, "refund_receipt_unverified")
	}
	if !allowRetry {
		return nil
	}
	if record.Status == FraudRefundReviewRequired {
		return nil
	}
	if !record.FirstAttemptAt.IsZero() &&
		(record.FirstAttemptAt.After(time.Now()) || time.Since(record.FirstAttemptAt) >= refundReplayWindow) {
		return s.reviewFraudRefund(record, "idempotency_window_expired")
	}
	if !normalizedSettings(stored.Settings).FraudRefundPolicy.Enabled {
		return s.stopFraudRefund(record, "policy_disabled")
	}
	return s.retryFraudRefund(ctx, record, stored, refunds)
}

func (s *Service) retryFraudRefund(
	ctx context.Context,
	record fraudRefundRecord,
	stored StoredState,
	refunds []*stripe.Refund,
) error {
	warning, err := s.stripe.Client().V1RadarEarlyFraudWarnings.Retrieve(ctx, record.WarningID, nil)
	if err != nil {
		return err
	}
	if warning.ID != record.WarningID || warning.Livemode || !warning.Actionable || warning.Charge == nil ||
		warning.Charge.ID != record.ChargeID {
		return s.reviewFraudRefund(record, "warning_not_actionable")
	}
	charge, err := s.stripe.Client().V1Charges.Retrieve(ctx, record.ChargeID, nil)
	if err != nil {
		return err
	}
	if chargeEligibility(charge, normalizedSettings(stored.Settings).FraudRefundPolicy) != "" ||
		charge.AmountCaptured-charge.AmountRefunded != record.Amount ||
		slices.ContainsFunc(refunds, func(refund *stripe.Refund) bool {
			return refund.Status == stripe.RefundStatusPending || refund.Status == stripe.RefundStatusRequiresAction
		}) {
		return s.reviewFraudRefund(record, "charge_changed")
	}
	owned, err := s.ownedRefundCharge(ctx, charge, stored)
	if err != nil {
		return err
	}
	if owned == nil || owned.InvoiceID != record.InvoiceID || owned.SubscriptionID != record.SubscriptionID ||
		owned.OrganizationID != record.OrganizationID || owned.CustomerID != record.CustomerID {
		return s.reviewFraudRefund(record, "ownership_unverified")
	}
	return s.issueFraudRefund(ctx, record)
}

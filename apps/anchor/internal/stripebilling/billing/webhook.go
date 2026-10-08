package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/stripe/stripe-go/v87"
	"github.com/stripe/stripe-go/v87/webhook"
)

// ReceiveWebhook validates signed Stripe input and commits its parsed inbox
// record and receipt-time policy without constructing a Stripe client.
func ReceiveWebhook(ctx context.Context, config Config, store StateStore, body []byte, signature string) error {
	if store == nil {
		return ErrInput
	}
	if err := validate.ValidateStruct(struct {
		WebhookSecret string `validate:"required,startswith=whsec_"`
		AccountID     string `validate:"required,startswith=acct_"`
	}{config.WebhookSecret, config.ExpectedAccountID}); err != nil {
		return ErrInput
	}
	if err := validate.ValidateStruct(struct {
		Signature string `validate:"required"`
	}{signature}); err != nil {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	event, err := webhook.ConstructEventWithOptions(
		body,
		signature,
		config.WebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true},
	)
	if err != nil {
		return fmt.Errorf("%w: invalid Stripe webhook signature", ErrInput)
	}
	if event.Livemode || event.Account != "" && event.Account != config.ExpectedAccountID {
		return fmt.Errorf("%w: webhook does not belong to the configured sandbox account", ErrInput)
	}
	var object struct {
		Customer string          `json:"customer"`
		ID       string          `json:"id"`
		Charge   json.RawMessage `json:"charge"`
	}
	if err = json.Unmarshal(event.Data.Raw, &object); err != nil {
		return ErrInput
	}
	chargeID := ""
	if len(object.Charge) != 0 && string(object.Charge) != "null" {
		var charge stripe.Charge
		if err = json.Unmarshal(object.Charge, &charge); err != nil {
			return ErrInput
		}
		chargeID = charge.ID
	}
	return store.Update(func(state *StoredState) error {
		if _, duplicate := state.Events[event.ID]; duplicate {
			return nil
		}
		organizationID := functional.Slice(slices.Collect(maps.Values(state.Organizations))).
			FindFirst(func(organization organizationRecord) bool {
				return object.Customer != "" && organization.CustomerID == object.Customer
			}).
			Map(func(organization organizationRecord) string { return organization.ID }).
			OrElse("")
		status := statusPending
		if organizationID == "" {
			status = "ignored"
		}
		if financialEvent(string(event.Type)) {
			if object.ID == "" || !validAnchorID(object.ID) || chargeID != "" && !validAnchorID(chargeID) {
				return ErrInput
			}
			status = statusPending
		}
		state.Events[event.ID] = eventRecord{
			ID:             event.ID,
			Type:           string(event.Type),
			OrganizationID: organizationID,
			ReceivedAt:     time.Now().UTC(),
			Status:         status,
			NextAttempt:    time.Now().UTC(),
			ResourceID:     object.ID,
			ChargeID:       chargeID,
			PolicyEnabled:  normalizedSettings(state.Settings).FraudRefundPolicy.Enabled,
		}
		return nil
	})
}

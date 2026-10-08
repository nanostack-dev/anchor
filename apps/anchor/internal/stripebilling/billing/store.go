package billing

import (
	"maps"
	"slices"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/segmentio/ksuid"
)

// HasUnresolvedRefunds prevents disconnecting an installation before financial
// receipts have been resolved. Unrecognized persisted statuses fail closed.
func HasUnresolvedRefunds(state StoredState) bool {
	return functional.Slice(slices.Collect(maps.Values(state.FraudRefunds))).
		FindFirst(func(record fraudRefundRecord) bool {
			return record.Status != FraudRefundSkipped && record.Status != FraudRefundSucceeded &&
				record.Status != FraudRefundFailed && record.Status != FraudRefundCanceled
		}).
		IsPresent()
}

type organizationRecord struct {
	Organization
	CustomerIntent string `json:"customer_intent"`
	CustomerName   string `json:"customer_name"`
	CheckoutIntent string `json:"checkout_intent"`
	CheckoutID     string `json:"checkout_id"`
	CheckoutPrice  string `json:"checkout_price"`
	CheckoutTrial  int    `json:"checkout_trial"`
}

type eventRecord struct {
	BillingEvent
	ResourceID    string    `json:"resource_id"`
	ChargeID      string    `json:"charge_id"`
	PolicyEnabled bool      `json:"policy_enabled"`
	Attempts      int       `json:"attempts"`
	NextAttempt   time.Time `json:"next_attempt"`
}

type fraudRefundRecord struct {
	FraudRefund
	IdempotencyKey string    `json:"idempotency_key"`
	FirstAttemptAt time.Time `json:"first_attempt_at"`
	InstallationID string    `json:"installation_id"`
	ProductID      string    `json:"product_id"`
	CustomerID     string    `json:"customer_id"`
}

func normalizedSettings(settings Settings) Settings {
	if settings.FraudRefundPolicy.Currency == "" {
		settings.FraudRefundPolicy = FraudRefundPolicy{Currency: FraudRefundCurrencyUsd}
	}
	return settings
}

type StoredState struct {
	InstallationID string                        `json:"installation_id"`
	AccountID      string                        `json:"account_id"`
	ProductID      string                        `json:"product_id"`
	Settings       Settings                      `json:"settings"`
	Prices         map[string]Price              `json:"prices"`
	Organizations  map[string]organizationRecord `json:"organizations"`
	Events         map[string]eventRecord        `json:"events"`
	PriceIntents   map[string]CreatePriceRequest `json:"price_intents"`
	StripeProducts map[string]string             `json:"stripe_products"`
	ProductNames   map[string]string             `json:"product_names"`
	FraudRefunds   map[string]fraudRefundRecord  `json:"fraud_refunds"`
}

// NewStoredState binds billing identity and all mutation intents to one product
// and Stripe account. Native storage persists this state in PostgreSQL.
func NewStoredState(accountID, productID string) StoredState {
	return StoredState{
		InstallationID: ksuid.New().String(),
		AccountID:      accountID,
		ProductID:      productID,
		Settings:       normalizedSettings(Settings{}),
		Prices:         map[string]Price{},
		Organizations:  map[string]organizationRecord{},
		Events:         map[string]eventRecord{},
		PriceIntents:   map[string]CreatePriceRequest{},
		StripeProducts: map[string]string{},
		ProductNames:   map[string]string{},
		FraudRefunds:   map[string]fraudRefundRecord{},
	}
}

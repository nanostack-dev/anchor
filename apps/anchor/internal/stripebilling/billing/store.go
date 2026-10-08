package billing

import (
	"time"

	"github.com/segmentio/ksuid"
)

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
	Attempts    int       `json:"attempts"`
	NextAttempt time.Time `json:"next_attempt"`
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
}

// NewStoredState binds billing identity and all mutation intents to one product
// and Stripe account. Native storage persists this state in PostgreSQL.
func NewStoredState(accountID, productID string) StoredState {
	return StoredState{
		InstallationID: ksuid.New().String(),
		AccountID:      accountID,
		ProductID:      productID,
		Settings:       Settings{},
		Prices:         map[string]Price{},
		Organizations:  map[string]organizationRecord{},
		Events:         map[string]eventRecord{},
		PriceIntents:   map[string]CreatePriceRequest{},
		StripeProducts: map[string]string{},
		ProductNames:   map[string]string{},
	}
}

package stripeprototype

import (
	"context"

	"anchor/internal/domain/integration"
)

type ConnectionConfig struct {
	AuthMethod    string
	APIKey        string
	AccountID     string
	WebhookSecret string
	ReturnURL     string
}

type ConnectionResolver interface {
	ResolveBillingConfig(context.Context, integration.Instance) (ConnectionConfig, error)
}

type StateStore interface {
	Snapshot() (StoredState, error)
	Update(func(*StoredState) error) error
}

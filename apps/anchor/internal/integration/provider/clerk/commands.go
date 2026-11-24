package clerk

import (
	"time"

	"anchor/internal/integration/provider"
)

// Command types specific to the Clerk provider.
const (
	CommandUpsertUser provider.CommandType = "UPSERT_USER"
	CommandDeleteUser provider.CommandType = "DELETE_USER"
)

// SyncSource identifies whether a command originated from a real-time Clerk
// webhook or a periodic reconciliation run.
type SyncSource string

const (
	SyncSourceWebhook   SyncSource = "webhook"
	SyncSourceReconcile SyncSource = "reconcile"
)

// UpsertUserData holds the data for upserting a user in a product.
type UpsertUserData struct {
	ExternalID    string
	Email         string
	EmailVerified bool
	Name          string
	EventAt       *time.Time
	Source        SyncSource
	Metadata      map[string]any
}

// DeleteUserData holds the data for deleting a user from a product.
type DeleteUserData struct {
	ExternalID string
	EventAt    *time.Time
}

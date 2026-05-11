package clerk

import "anchor/internal/integration/provider"

// Command types specific to the Clerk provider.
const (
	CommandUpsertUser provider.CommandType = "UPSERT_USER"
	CommandDeleteUser provider.CommandType = "DELETE_USER"
)

// UpsertUserData holds the data for upserting a user in a product.
type UpsertUserData struct {
	ExternalID string
	Email      string
	Name       string
	Metadata   map[string]any
}

// DeleteUserData holds the data for deleting a user from a product.
type DeleteUserData struct {
	ExternalID string
}

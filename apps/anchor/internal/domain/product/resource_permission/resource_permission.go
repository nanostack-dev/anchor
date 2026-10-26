package resourcepermission

import (
	"time"
)

// ProductResourcePermission represents a resource permission defined within a specific product
// Managed by anchor admins to define granular permissions (e.g., file:read, document:edit).
type ProductResourcePermission struct {
	ProductID      string
	Name           string
	Description    *string
	ScopeModifier  *string
	ResourceTypeID string
	// ResourceType is the slug of the scope, filled by the service.
	ResourceType string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

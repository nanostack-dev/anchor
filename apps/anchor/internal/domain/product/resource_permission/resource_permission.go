package resourcepermission

import (
	"time"
)

// ProductResourcePermission represents a resource permission defined within a specific product
// Managed by anchor admins to define granular permissions (e.g., file:read, document:edit).
type ProductResourcePermission struct {
	ProductID     string
	Name          string
	Description   *string
	ScopeModifier *string
	// ResourceTypeID is nil only on a row a release before resource types
	// wrote during a rolling deploy; it counts as organization.
	ResourceTypeID *string
	// ResourceType is the slug of the scope, filled by the service.
	ResourceType string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

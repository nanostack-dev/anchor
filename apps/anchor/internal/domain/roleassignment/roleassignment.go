// Package roleassignment holds the roles members hold on registered resources
// (ADR-0022 D3, D4). An assignment names one resource, one member of its
// Organization and one of the Organization's roles, scoped to the resource's
// type. The contract is list-shaped: a member holds a list of roles on a
// resource, and a product allows one role per member per resource for now.
package roleassignment

import "time"

const (
	// MaxRolesPerResource is how many roles a member holds on one resource
	// while the product allows one role per resource, the only setting so far
	// (ADR-0022 Q-D3.2).
	MaxRolesPerResource = 1
)

// Role is a role held on a resource: its row, its public ID and its name.
type Role struct {
	ID   string
	Name string
}

// Member is a member of the Organization and the roles they hold on one
// resource.
type Member struct {
	ProductUserID string
	Roles         []Role
	// CreatedAt is when the member first held a role on the resource,
	// UpdatedAt when their roles there last changed.
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Held is one assignment with what its events name: the resource as the
// product names it, and the role.
type Held struct {
	ResourceRowID      string
	ResourceType       string
	ResourceExternalID string
	OrganizationID     string
	ProductUserID      string
	RoleID             string
}

// Grant is a role a member holds on a resource that holds one permission,
// as an access decision explains it.
type Grant struct {
	ResourceRowID string
	Role          Role
	Permission    string
}

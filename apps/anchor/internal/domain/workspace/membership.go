package workspace

import "time"

// Membership is the role an organization member holds in one workspace, on top
// of their organization role.
type Membership struct {
	WorkspaceID    string
	OrganizationID string
	ProductUserID  string
	Email          string
	Name           *string
	RoleID         string
	RoleName       string
	AssignedAt     time.Time
	UpdatedAt      time.Time
}

// SortFieldMember defines valid sort fields for workspace member search results.
type SortFieldMember string

const (
	SortFieldMemberAssignedAt SortFieldMember = "assigned_at"
	SortFieldMemberEmail      SortFieldMember = "email"
)

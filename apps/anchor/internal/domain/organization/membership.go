package organization

import "time"

// WithoutRolePermissions excludes permission details from membership reads and responses.
const WithoutRolePermissions = false

// Membership represents a product user's membership within an organization,
// including their role and permissions within that organization.
type Membership struct {
	OrganizationID  string
	ProductUserID   string
	UserEmail       string
	UserName        string
	UserExternalID  *string
	RoleID          string
	RoleName        string
	RolePermissions []string
	JoinedAt        time.Time
}

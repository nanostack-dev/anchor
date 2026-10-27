package organization

import (
	"time"

	"anchor/internal/domain/product/role"
)

// WithoutRolePermissions excludes permission details from membership reads and responses.
const WithoutRolePermissions = false

// Membership represents a product user's membership within an organization,
// including their role and permissions within that organization. Role is nil
// for a member who holds no role, and then RolePermissions is empty.
type Membership struct {
	OrganizationID  string
	ProductUserID   string
	UserEmail       string
	UserName        string
	UserExternalID  *string
	Role            *role.Ref
	RolePermissions []string
	JoinedAt        time.Time
}

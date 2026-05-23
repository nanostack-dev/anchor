package platform

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// TenantRole defines the role of a user within a tenant.
type TenantRole string

const (
	// TenantRoleOwner has full control over the tenant. Typically the first user.
	TenantRoleOwner TenantRole = "OWNER"
	// TenantRoleAdmin has administrative privileges within the tenant.
	TenantRoleAdmin TenantRole = "ADMIN"
)

func (r TenantRole) ToString() string {
	return string(r)
}

type User struct {
	// User fields
	ID             string
	UserID         string
	ExternalID     *string
	Name           string
	Email          string
	HashedPassword string
	CreatedAt      time.Time
	UpdatedAt      time.Time

	// Platform tenant membership fields
	PlatformTenantID string
	Role             TenantRole
}

// GenerateID sets the platform user's ID to a new prefixed KSUID.
func (u *User) GenerateID() {
	u.ID = ids.MustNew("puser")
}

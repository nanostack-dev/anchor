package user

import (
	"encoding/json"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"anchor/internal/domain/product/role"
)

type ProductUserStatus string

const (
	ProductUserStatusActive   ProductUserStatus = "ACTIVE"
	ProductUserStatusInactive ProductUserStatus = "INACTIVE"
)

type ProductUser struct {
	ID         string
	ProductID  string
	Email      string
	Name       string
	ExternalID *string
	Status     ProductUserStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// GenerateID sets the product user's ID to a new prefixed KSUID.
func (u *ProductUser) GenerateID() {
	u.ID = ids.MustNew("pusr")
}

// OrganizationMembership represents an organization from the user's perspective,
// including their role and membership details within that organization. Role
// is nil when the user holds no role there.
type OrganizationMembership struct {
	OrganizationID           string
	OrganizationName         string
	OrganizationDescription  *string
	OrganizationMetadataJSON json.RawMessage
	Role                     *role.Ref
	RolePermissions          []string // Only populated when include=role_permissions
	JoinedAt                 time.Time
}

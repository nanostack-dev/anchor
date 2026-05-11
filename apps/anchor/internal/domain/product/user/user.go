package user

import (
	"time"

	"github.com/nanostack-dev/shared/toolkit"
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
	u.ID = toolkit.NewID("pusr")
}

// OrganizationMembership represents an organization from the user's perspective,
// including their role and membership details within that organization.
type OrganizationMembership struct {
	OrganizationID          string
	OrganizationName        string
	OrganizationDescription *string
	RoleID                  string
	RoleName                string
	RolePermissions         []string // Only populated when include=role_permissions
	JoinedAt                time.Time
}

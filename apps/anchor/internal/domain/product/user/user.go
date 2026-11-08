package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"anchor/internal/domain/product/role"
)

// ProductUserStatus is stored and returned in the contract's spelling: the
// ProductUserStatus enum in cmd/http/openapi.yaml.
type ProductUserStatus string

const (
	ProductUserStatusActive    ProductUserStatus = "active"
	ProductUserStatusInvited   ProductUserStatus = "invited"
	ProductUserStatusSuspended ProductUserStatus = "suspended"
)

// ErrUnknownProductUserStatus is returned by ParseProductUserStatus for a value
// outside the contract enum.
var ErrUnknownProductUserStatus = errors.New("unknown product user status")

// ParseProductUserStatus returns the status that value names. Letter case is
// ignored: until 2026-11-08 Anchor stored and returned ACTIVE, so clients may
// send it back, and replicas of the previous release write it during a deploy.
// Any other value returns ErrUnknownProductUserStatus.
func ParseProductUserStatus(value string) (ProductUserStatus, error) {
	status := ProductUserStatus(strings.ToLower(value))
	if !status.Valid() {
		return "", fmt.Errorf("%w %q", ErrUnknownProductUserStatus, value)
	}
	return status, nil
}

// Valid reports whether s is a status of the contract enum, in its spelling.
func (s ProductUserStatus) Valid() bool {
	switch s {
	case ProductUserStatusActive, ProductUserStatusInvited, ProductUserStatusSuspended:
		return true
	default:
		return false
	}
}

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

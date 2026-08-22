package apikey

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// Status represents the state of a product API key.
type Status string

// API key status constants.
const (
	StatusActive   Status = "ACTIVE"
	StatusInactive Status = "INACTIVE"
)

// ProductAPIKey represents an API key for a product.
type ProductAPIKey struct {
	ID              string
	ProductID       string
	Name            string
	Description     *string
	Mutable         bool
	HashedValue     string
	ObfuscatedValue string
	Status          Status
	LastUsedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Permissions     []ProductAPIKeyPermission
}

// ProductAPIKeyPermission represents a permission assigned to an API key.
type ProductAPIKeyPermission struct {
	APIKeyID       string
	ProductID      string
	PermissionName string
	CreatedAt      time.Time
}

// GenerateID sets the API key's ID to a new prefixed KSUID.
func (s *ProductAPIKey) GenerateID() {
	s.ID = ids.MustNew("product_apikey")
}

func (s *ProductAPIKey) ToStringsPermissions() []string {
	return functional.Slice(
		s.Permissions).Map(
		func(perm ProductAPIKeyPermission) string {
			return perm.PermissionName
		})
}

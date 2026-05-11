package orgapikey

import (
	"time"

	"github.com/nanostack-dev/shared/toolkit"
)

type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusInactive Status = "INACTIVE"
)

type OrganizationAPIKey struct {
	ID              string
	OrganizationID  string
	Name            string
	Description     *string
	HashedValue     string
	ObfuscatedValue string
	Status          Status
	ExpiresAt       *time.Time
	LastUsedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Permissions     []OrganizationAPIKeyPermission
}

type OrganizationAPIKeyPermission struct {
	APIKeyID       string
	OrganizationID string
	ProductID      string
	PermissionName string
	CreatedAt      time.Time
}

func (s *OrganizationAPIKey) GenerateID() {
	s.ID = toolkit.NewID("organization_apikey")
}

func (s *OrganizationAPIKey) IsExpiredAt(now time.Time) bool {
	if s.ExpiresAt == nil {
		return false
	}

	return !s.ExpiresAt.After(now)
}

package organizationinvitation

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

const DefaultExpiry = 7 * 24 * time.Hour

type Status string

const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
	StatusExpired  Status = "expired"
)

type Invitation struct {
	ID             string
	ProductID      string
	OrganizationID string
	Email          string
	RoleID         string
	TokenHash      string
	Status         Status
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (i *Invitation) GenerateID() {
	i.ID = ids.MustNew("oinv")
}

func DeriveStatus(acceptedAt *time.Time, expiresAt, now time.Time) Status {
	if acceptedAt != nil {
		return StatusAccepted
	}
	if !expiresAt.After(now) {
		return StatusExpired
	}
	return StatusPending
}

func (i *Invitation) StatusAt(now time.Time) Status {
	return DeriveStatus(i.AcceptedAt, i.ExpiresAt, now)
}

func DefaultExpiryFrom(now time.Time) time.Time {
	return now.Add(DefaultExpiry)
}

// Created is an invitation together with the clear token, which exists only in
// the response of the call that made or replaced it.
type Created struct {
	Invitation Invitation
	Token      string
}

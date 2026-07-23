// Package license defines per-organization licenses, license signing keys and
// signed license-token claims.
//
// A license row is the mutable source of truth (plan, status, expiry, grace,
// overrides); signed tokens are short-lived derived snapshots of it — never
// the license itself.
package license

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"anchor/internal/domain/plan"
)

// Status is the mutable lifecycle status of a license row.
type Status string

const (
	StatusActive    Status = "ACTIVE"
	StatusSuspended Status = "SUSPENDED"
	StatusRevoked   Status = "REVOKED"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusActive, StatusSuspended, StatusRevoked:
		return true
	default:
		return false
	}
}

// DefaultTokenTTLSeconds is the token lifetime applied when a license does not
// override it, and for tokens issued from a product's default plan.
const DefaultTokenTTLSeconds int32 = 86400

type License struct {
	ID                   string
	ProductID            string
	OrganizationID       string
	PlanID               string
	Status               Status
	ExpiresAt            *time.Time
	GraceUntil           *time.Time
	EntitlementOverrides plan.Entitlements
	TokenTTLSeconds      int32
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// GenerateID sets the license's ID to a new prefixed KSUID.
func (l *License) GenerateID() {
	l.ID = ids.MustNew("lic")
}

// GraceBoundary is the instant after which no token may be issued anymore: the
// business grace window if set, otherwise the expiry itself. Returns nil for
// licenses without expiry.
func (l *License) GraceBoundary() *time.Time {
	if l.GraceUntil != nil {
		return l.GraceUntil
	}

	return l.ExpiresAt
}

// ResolvedEntitlements merges the plan's entitlements with this license's
// overrides (override wins).
func (l *License) ResolvedEntitlements(planEntitlements plan.Entitlements) plan.Entitlements {
	return planEntitlements.MergedWith(l.EntitlementOverrides)
}

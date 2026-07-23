// Package license defines per-organization licenses and the resolved
// entitlement snapshot consumer services read from Anchor.
//
// The license row is the mutable source of truth (plan, status, expiry, grace,
// overrides). A snapshot is a derived read of it — never the license itself.
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

// EffectiveStatus is the status reported on an entitlement snapshot. Unlike the
// stored license status it carries GRACE (past expiry, still inside the
// business grace window) and has no REVOKED value: revoked licenses resolve to
// no snapshot at all.
type EffectiveStatus string

const (
	EffectiveStatusActive    EffectiveStatus = "ACTIVE"
	EffectiveStatusGrace     EffectiveStatus = "GRACE"
	EffectiveStatusSuspended EffectiveStatus = "SUSPENDED"
)

// DefaultRefreshIntervalSeconds is the re-read cadence applied when a license
// does not override it, and for snapshots resolved from a product's default
// plan.
const DefaultRefreshIntervalSeconds int32 = 86400

type License struct {
	ID                     string
	ProductID              string
	OrganizationID         string
	PlanID                 string
	Status                 Status
	ExpiresAt              *time.Time
	GraceUntil             *time.Time
	EntitlementOverrides   plan.Entitlements
	RefreshIntervalSeconds int32
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// GenerateID sets the license's ID to a new prefixed KSUID.
func (l *License) GenerateID() {
	l.ID = ids.MustNew("lic")
}

// GraceBoundary is the instant after which the license no longer resolves to a
// usable snapshot: the business grace window if set, otherwise the expiry
// itself. Returns nil for licenses without expiry.
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

// EntitlementSnapshot is the resolved read of an organization's license:
// plan entitlements merged with per-org overrides, the effective status, and
// the instant after which the consumer should read again.
type EntitlementSnapshot struct {
	OrganizationID string
	ProductID      string
	PlanKey        string
	Status         EffectiveStatus
	Entitlements   plan.Entitlements
	ExpiresAt      *time.Time
	GraceUntil     *time.Time
	RefreshAfter   time.Time
}

// Package plan defines product plans and their entitlement maps.
//
// A plan is a per-product bundle of entitlements identified by a stable `key`
// (the future Stripe lookup_key). Entitlement resolution merges plan defaults
// with per-license overrides (override wins) — see Entitlements.MergedWith.
package plan

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

type Plan struct {
	ID           string
	ProductID    string
	Key          string
	Name         string
	Description  string
	Entitlements Entitlements
	IsDefault    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GenerateID sets the plan's ID to a new prefixed KSUID.
func (p *Plan) GenerateID() {
	p.ID = ids.MustNew("plan")
}

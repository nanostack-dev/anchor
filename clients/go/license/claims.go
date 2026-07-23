package license

import "time"

// Token statuses embedded in the signed claims by Anchor.
const (
	tokenStatusGrace     = "GRACE"
	tokenStatusSuspended = "SUSPENDED"
)

// Entitlement value types.
const (
	entitlementTypeBoolean = "boolean"
	entitlementTypeNumeric = "numeric"
)

// EntitlementValue is a single resolved entitlement from the token snapshot:
// a boolean feature gate or a numeric limit.
type EntitlementValue struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// Claims is the verified payload of an Anchor license token. Instances are
// only ever produced by Verifier after the Ed25519 signature checked out —
// never parse token payloads yourself.
type Claims struct {
	OrganizationID string
	ProductID      string
	PlanKey        string
	// Status is the status embedded at issuance time: ACTIVE, GRACE or
	// SUSPENDED. Prefer the Status returned by Verify, which also accounts for
	// expiry observed at verification time.
	Status        string
	Entitlements  map[string]EntitlementValue
	IssuedAt      time.Time
	ExpiresAt     time.Time
	GraceUntil    *time.Time
	RefreshAfter  time.Time
	SchemaVersion int
}

// HasFeature reports whether the boolean entitlement `key` is granted.
// Missing keys and non-boolean entitlements are false.
func (c *Claims) HasFeature(key string) bool {
	entitlement, ok := c.Entitlements[key]
	if !ok || entitlement.Type != entitlementTypeBoolean {
		return false
	}

	value, ok := entitlement.Value.(bool)
	return ok && value
}

// Limit returns the numeric entitlement `key`. ok is false for missing keys
// and non-numeric entitlements.
func (c *Claims) Limit(key string) (float64, bool) {
	entitlement, ok := c.Entitlements[key]
	if !ok || entitlement.Type != entitlementTypeNumeric {
		return 0, false
	}

	value, ok := entitlement.Value.(float64)
	if !ok {
		return 0, false
	}

	return value, true
}

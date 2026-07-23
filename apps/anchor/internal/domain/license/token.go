package license

import (
	"time"

	"anchor/internal/domain/plan"
)

// TokenStatus is the status carried inside a signed license token. Unlike the
// license row status it has no REVOKED value: revoked licenses simply stop
// getting tokens.
type TokenStatus string

const (
	TokenStatusActive    TokenStatus = "ACTIVE"
	TokenStatusGrace     TokenStatus = "GRACE"
	TokenStatusSuspended TokenStatus = "SUSPENDED"
)

// ClaimsSchemaVersion is the current license token claim schema version.
const ClaimsSchemaVersion = 1

// Claim keys used in the PASETO token payload and footer. iat/exp use the
// PASETO registered claim names.
const (
	ClaimOrganizationID = "organization_id"
	ClaimProductID      = "product_id"
	ClaimPlanKey        = "plan_key"
	ClaimStatus         = "status"
	ClaimEntitlements   = "entitlements"
	ClaimGraceUntil     = "grace_until"
	ClaimRefreshAfter   = "refresh_after"
	ClaimSchemaVersion  = "schema_version"
	FooterKid           = "kid"
)

// Claims is the payload of a signed license token: a derived, disposable
// snapshot of the license row (resolved entitlements included), never the
// license itself.
type Claims struct {
	OrganizationID string
	ProductID      string
	PlanKey        string
	Status         TokenStatus
	Entitlements   plan.Entitlements
	IssuedAt       time.Time
	ExpiresAt      time.Time
	GraceUntil     *time.Time
	RefreshAfter   time.Time
	SchemaVersion  int
}

// IssuedToken is the result of a token issuance: the signed token plus the
// refresh hints consumers schedule against.
type IssuedToken struct {
	Token        string
	RefreshAfter time.Time
	ExpiresAt    time.Time
}

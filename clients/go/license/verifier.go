// Package license is the canonical offline verifier for Anchor license
// tokens (PASETO v4.public, Ed25519).
//
// Consumers fetch the product's signing keys once at startup
// (GET /v1/products/{product_id}/license-signing-keys), construct a Verifier,
// and verify the cached license token locally on every check — no network
// call, sub-microsecond. Claims are unreachable unless the signature
// verified: parse-before-verify is the classic licensing exploit and this
// package makes it impossible by construction.
package license

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	paseto "aidanwoods.dev/go-paseto"
)

// Status is the machine-readable verification outcome. It is deliberately a
// typed status, not a boolean: "expired — show renew banner" and "suspended —
// hard block" are different UX.
type Status string

const (
	// StatusValid means the signature verified, the token is unexpired and
	// the license is active.
	StatusValid Status = "VALID"
	// StatusGrace means the signature verified but the license is in a grace
	// window — either issued with status GRACE (business grace), past exp but
	// within the token's grace_until, or past exp within the caller's sync
	// grace. Degrade gracefully; schedule a refresh.
	StatusGrace Status = "GRACE"
	// StatusExpired means the signature verified but the token is past every
	// grace boundary. Fetch a fresh token.
	StatusExpired Status = "EXPIRED"
	// StatusSuspended means the signature verified, the token is unexpired
	// and the license is suspended. Hard-block gated functionality.
	StatusSuspended Status = "SUSPENDED"
	// StatusInvalid means the token could not be cryptographically verified
	// (bad signature, unknown kid, wrong format, malformed claims). Claims
	// are nil.
	StatusInvalid Status = "INVALID"
)

const (
	v4PublicHeader = "v4.public."
	footerKidField = "kid"

	claimOrganizationID = "organization_id"
	claimProductID      = "product_id"
	claimPlanKey        = "plan_key"
	claimStatus         = "status"
	claimEntitlements   = "entitlements"
	claimGraceUntil     = "grace_until"
	claimRefreshAfter   = "refresh_after"
	claimSchemaVersion  = "schema_version"
)

// PublicKey is one trusted verification key as served by
// GET /v1/products/{product_id}/license-signing-keys.
type PublicKey struct {
	// Kid is the key identifier matched against the token footer.
	Kid string
	// Key is the base64 (std encoding) raw Ed25519 public key.
	Key string
}

// Verifier verifies Anchor license tokens against a pinned set of trusted
// public keys. It is immutable and safe for concurrent use.
type Verifier struct {
	keys map[string]paseto.V4AsymmetricPublicKey
}

// NewVerifier builds a Verifier from the trusted key set. At least one key is
// required; duplicate kids are rejected.
func NewVerifier(keys []PublicKey) (*Verifier, error) {
	if len(keys) == 0 {
		return nil, errors.New("license: at least one public key is required")
	}

	parsed := make(map[string]paseto.V4AsymmetricPublicKey, len(keys))
	for _, key := range keys {
		if key.Kid == "" {
			return nil, errors.New("license: public key with empty kid")
		}
		if _, exists := parsed[key.Kid]; exists {
			return nil, fmt.Errorf("license: duplicate kid %q", key.Kid)
		}

		raw, err := base64.StdEncoding.DecodeString(key.Key)
		if err != nil {
			return nil, fmt.Errorf("license: kid %q: invalid base64 public key: %w", key.Kid, err)
		}
		publicKey, err := paseto.NewV4AsymmetricPublicKeyFromBytes(raw)
		if err != nil {
			return nil, fmt.Errorf("license: kid %q: invalid ed25519 public key: %w", key.Kid, err)
		}

		parsed[key.Kid] = publicKey
	}

	return &Verifier{keys: parsed}, nil
}

// Verify checks the token strictly: past-exp tokens are only accepted within
// the token's own grace_until (returning StatusGrace). Equivalent to
// VerifyWithGrace(token, 0).
func (v *Verifier) Verify(token string) (*Claims, Status, error) {
	return v.VerifyWithGrace(token, 0)
}

// VerifyWithGrace verifies the token and additionally tolerates exp being
// exceeded by up to syncGrace (the offline/sync grace window: Anchor being
// unreachable at refresh time must degrade, not crash). Grace from either
// source yields StatusGrace.
//
// Claims are non-nil only when the signature verified; for StatusInvalid they
// are always nil.
func (v *Verifier) VerifyWithGrace(
	token string, syncGrace time.Duration,
) (*Claims, Status, error) {
	if !strings.HasPrefix(token, v4PublicHeader) {
		return nil, StatusInvalid, errors.New("license: token is not PASETO v4.public")
	}

	kid, err := extractKid(token)
	if err != nil {
		return nil, StatusInvalid, err
	}

	publicKey, ok := v.keys[kid]
	if !ok {
		return nil, StatusInvalid, fmt.Errorf("license: unknown kid %q", kid)
	}

	// Expiry is evaluated below so grace windows can apply; the signature and
	// claim shape are still fully verified here.
	parsed, err := paseto.NewParserWithoutExpiryCheck().ParseV4Public(publicKey, token, nil)
	if err != nil {
		return nil, StatusInvalid, fmt.Errorf("license: token verification failed: %w", err)
	}

	claims, err := claimsFromToken(parsed)
	if err != nil {
		return nil, StatusInvalid, err
	}

	return claims, resolveStatus(claims, time.Now(), syncGrace), nil
}

// extractKid reads the kid from the (unauthenticated) token footer. The
// footer only selects which trusted key to try — a forged footer can at worst
// select a key the signature then fails against.
func extractKid(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[3] == "" {
		return "", errors.New("license: token has no footer")
	}

	rawFooter, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return "", fmt.Errorf("license: invalid token footer encoding: %w", err)
	}

	var footer map[string]string
	if unmarshalErr := json.Unmarshal(rawFooter, &footer); unmarshalErr != nil {
		return "", fmt.Errorf("license: invalid token footer: %w", unmarshalErr)
	}
	kid := footer[footerKidField]
	if kid == "" {
		return "", errors.New("license: token footer has no kid")
	}

	return kid, nil
}

func claimsFromToken(token *paseto.Token) (*Claims, error) {
	claims := &Claims{}

	var err error
	if claims.OrganizationID, err = token.GetString(claimOrganizationID); err != nil {
		return nil, fmt.Errorf("license: missing organization_id claim: %w", err)
	}
	if claims.ProductID, err = token.GetString(claimProductID); err != nil {
		return nil, fmt.Errorf("license: missing product_id claim: %w", err)
	}
	if claims.PlanKey, err = token.GetString(claimPlanKey); err != nil {
		return nil, fmt.Errorf("license: missing plan_key claim: %w", err)
	}
	if claims.Status, err = token.GetString(claimStatus); err != nil {
		return nil, fmt.Errorf("license: missing status claim: %w", err)
	}
	if err = token.Get(claimEntitlements, &claims.Entitlements); err != nil {
		return nil, fmt.Errorf("license: missing entitlements claim: %w", err)
	}
	if claims.IssuedAt, err = token.GetIssuedAt(); err != nil {
		return nil, fmt.Errorf("license: missing iat claim: %w", err)
	}
	if claims.ExpiresAt, err = token.GetExpiration(); err != nil {
		return nil, fmt.Errorf("license: missing exp claim: %w", err)
	}
	if claims.RefreshAfter, err = token.GetTime(claimRefreshAfter); err != nil {
		return nil, fmt.Errorf("license: missing refresh_after claim: %w", err)
	}
	if err = token.Get(claimSchemaVersion, &claims.SchemaVersion); err != nil {
		return nil, fmt.Errorf("license: missing schema_version claim: %w", err)
	}

	// grace_until is optional: absent on tokens without a business grace
	// window (e.g. default-plan tokens).
	if graceUntil, graceErr := token.GetTime(claimGraceUntil); graceErr == nil {
		claims.GraceUntil = &graceUntil
	}

	return claims, nil
}

// resolveStatus maps verified claims onto the verification-time status.
// Expiry is evaluated first: an expired snapshot is stale regardless of the
// status it carried at issuance.
func resolveStatus(claims *Claims, now time.Time, syncGrace time.Duration) Status {
	if now.After(claims.ExpiresAt) {
		withinBusinessGrace := claims.GraceUntil != nil && !now.After(*claims.GraceUntil)
		withinSyncGrace := syncGrace > 0 && !now.After(claims.ExpiresAt.Add(syncGrace))
		if withinBusinessGrace || withinSyncGrace {
			return StatusGrace
		}

		return StatusExpired
	}

	switch claims.Status {
	case tokenStatusSuspended:
		return StatusSuspended
	case tokenStatusGrace:
		return StatusGrace
	default:
		return StatusValid
	}
}

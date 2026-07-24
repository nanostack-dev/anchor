package webhook

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// SecretStatus tracks a signing secret through rotation.
type SecretStatus string

const (
	SecretStatusActive   SecretStatus = "ACTIVE"
	SecretStatusExpiring SecretStatus = "EXPIRING"
)

func (s SecretStatus) IsValid() bool {
	switch s {
	case SecretStatusActive, SecretStatusExpiring:
		return true
	default:
		return false
	}
}

// SigningPrefix marks a webhook signing secret so a leaked value is
// immediately identifiable in a log or a support ticket.
//
// It is the Standard Webhooks prefix rather than an Anchor-branded one on
// purpose: consumer libraries recognise `whsec_`, strip it, and base64-decode
// the remainder. Anchor's own `PrefixedSpec` tokens (checksummed alphanumerics,
// used for API keys) are deliberately NOT used here — they are not decodable,
// so an off-the-shelf verifier would derive the wrong HMAC key.
const SigningPrefix = "whsec_"

// SecretRandomBytes is the entropy behind a generated signing secret. The spec
// allows 24-64 bytes; 32 matches the HMAC-SHA256 block size.
const SecretRandomBytes = 32

// SecretRotationGrace is how long the previous secret keeps signing alongside
// the new one, so a receiver can roll over without coordination or downtime.
const SecretRotationGrace = 24 * time.Hour

// GenerateSecret returns a fresh plaintext signing secret in Standard Webhooks
// form: `whsec_` followed by base64-encoded random bytes.
func GenerateSecret() (string, error) {
	raw := make([]byte, SecretRandomBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate webhook signing secret: %w", err)
	}

	return SigningPrefix + base64.StdEncoding.EncodeToString(raw), nil
}

// SigningKey returns the HMAC key for a plaintext secret: the `whsec_` prefix
// stripped and the remainder base64-decoded, exactly as a consumer library
// does. A secret that does not decode is rejected rather than silently signed
// with its literal bytes, which would produce signatures no verifier accepts.
func SigningKey(plaintextSecret string) ([]byte, error) {
	encoded := strings.TrimPrefix(plaintextSecret, SigningPrefix)

	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode webhook signing secret: %w", err)
	}

	return key, nil
}

// Secret is one signing secret of an endpoint. Two rows co-exist during
// rotation: the new ACTIVE one and the previous EXPIRING one.
type Secret struct {
	ID              string
	EndpointID      string
	EncryptedSecret string
	Status          SecretStatus
	ExpiresAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// GenerateID sets the secret's ID to a new prefixed KSUID.
func (s *Secret) GenerateID() {
	s.ID = ids.MustNew("whs")
}

// IsUsable reports whether this secret still contributes a signature.
// ACTIVE always does; EXPIRING does until its expiry passes.
func (s *Secret) IsUsable(now time.Time) bool {
	switch s.Status {
	case SecretStatusActive:
		return true
	case SecretStatusExpiring:
		return s.ExpiresAt == nil || now.Before(*s.ExpiresAt)
	default:
		return false
	}
}

// UsableSecrets filters a secret set down to the ones that should sign an
// attempt made at `now`.
func UsableSecrets(all []Secret, now time.Time) []Secret {
	usable := make([]Secret, 0, len(all))
	for _, secret := range all {
		if secret.IsUsable(now) {
			usable = append(usable, secret)
		}
	}

	return usable
}

package webhook

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
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
const SigningPrefix = "anchor_whsec_"

// SecretRandomLength is the random body length of a generated signing secret.
const SecretRandomLength = 48

// SecretRotationGrace is how long the previous secret keeps signing alongside
// the new one, so a receiver can roll over without coordination or downtime.
const SecretRotationGrace = 24 * time.Hour

// SecretSpec generates and validates webhook signing secrets.
func SecretSpec() secrets.PrefixedSpec {
	return secrets.PrefixedSpec{
		Prefix:       SigningPrefix,
		RandomLength: SecretRandomLength,
	}
}

// GenerateSecret returns a fresh plaintext signing secret.
func GenerateSecret() (string, error) {
	return SecretSpec().Generate()
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

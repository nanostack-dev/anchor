package license

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// SigningKeyStatus models the key-rotation lifecycle: ACTIVE keys sign new
// tokens, RETIRING keys still verify but no longer sign, RETIRED keys are kept
// for audit only.
type SigningKeyStatus string

const (
	SigningKeyStatusActive   SigningKeyStatus = "ACTIVE"
	SigningKeyStatusRetiring SigningKeyStatus = "RETIRING"
	SigningKeyStatusRetired  SigningKeyStatus = "RETIRED"
)

func (s SigningKeyStatus) IsValid() bool {
	switch s {
	case SigningKeyStatusActive, SigningKeyStatusRetiring, SigningKeyStatusRetired:
		return true
	default:
		return false
	}
}

// SigningKey is a deployment-global Ed25519 keypair used to sign PASETO
// v4.public license tokens. ID doubles as the token `kid` footer value.
// PrivateKeyEncrypted is a framework VersionedCipher ciphertext; the raw
// private key exists only in memory at signing time.
type SigningKey struct {
	ID                  string
	PublicKey           string // base64 (std encoding) raw ed25519 public key
	PrivateKeyEncrypted string
	Status              SigningKeyStatus
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// GenerateID sets the signing key's ID (the token kid) to a new prefixed KSUID.
func (k *SigningKey) GenerateID() {
	k.ID = ids.MustNew("lsk")
}

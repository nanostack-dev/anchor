package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	paseto "aidanwoods.dev/go-paseto"
	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	"anchor/internal/domain/plan"
	"anchor/internal/repository"
	"anchor/internal/security/encryption"
)

// licenseSigningKeyCipherContext scopes the VersionedCipher used for signing
// keys so their ciphertexts are cryptographically isolated from other secret
// classes (same pattern as integration secrets).
const licenseSigningKeyCipherContext = "license-signing-key"

var errLicenseSigningEncryptionMissing = errors.New(
	"global encryption key is not configured for license signing keys",
)

// LicenseSigningService signs license claims into PASETO v4.public tokens and
// manages the Ed25519 signing keypair lifecycle. Private keys are stored
// VersionedCipher-encrypted and decrypted only in memory at signing time.
type LicenseSigningService interface {
	Sign(ctx context.Context, claims license.Claims) (string, error)
	// EnsureActiveSigningKey guarantees one ACTIVE signing key exists,
	// generating and persisting a new Ed25519 keypair when none does.
	EnsureActiveSigningKey(ctx context.Context) (license.SigningKey, error)
}

type licenseSigningService struct {
	keyRepo   repository.LicenseSigningKeyRepository
	cipher    *secrets.VersionedCipher
	cipherErr error
	logger    zerolog.Logger
}

func NewLicenseSigningService(
	keyRepo repository.LicenseSigningKeyRepository,
	encryptionService *encryption.Service,
	logger zerolog.Logger,
) LicenseSigningService {
	var (
		cipher *secrets.VersionedCipher
		err    error
	)
	if encryptionService == nil {
		err = errLicenseSigningEncryptionMissing
	} else {
		cipher, err = encryptionService.NewCipher(licenseSigningKeyCipherContext)
	}

	return &licenseSigningService{
		keyRepo:   keyRepo,
		cipher:    cipher,
		cipherErr: err,
		logger:    logger.With().Str("component", "license_signing_service").Logger(),
	}
}

func (s *licenseSigningService) Sign(
	ctx context.Context, claims license.Claims,
) (string, error) {
	if s.cipherErr != nil {
		return "", fmt.Errorf(
			"failed to initialize license signing cipher: %w", s.cipherErr,
		)
	}

	key, err := s.keyRepo.FindActive(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to load active license signing key")
		return "", err
	}
	if key == nil {
		return "", ErrLicenseSigningKeyMissing
	}

	secretKey, err := s.decryptSecretKey(*key)
	if err != nil {
		s.logger.Error().Str("kid", key.ID).Err(err).Msg("failed to decrypt license signing key")
		return "", err
	}

	return signClaims(claims, secretKey, key.ID)
}

func (s *licenseSigningService) EnsureActiveSigningKey(
	ctx context.Context,
) (license.SigningKey, error) {
	if s.cipherErr != nil {
		return license.SigningKey{}, fmt.Errorf(
			"failed to initialize license signing cipher: %w", s.cipherErr,
		)
	}

	existing, err := s.keyRepo.FindActive(ctx)
	if err != nil {
		return license.SigningKey{}, err
	}
	if existing != nil {
		return *existing, nil
	}

	secretKey := paseto.NewV4AsymmetricSecretKey()
	encryptedPrivate, err := s.cipher.EncryptString(
		base64.StdEncoding.EncodeToString(secretKey.ExportBytes()),
	)
	if err != nil {
		return license.SigningKey{}, fmt.Errorf(
			"failed to encrypt license signing private key: %w", err,
		)
	}

	key := license.SigningKey{
		PublicKey: base64.StdEncoding.EncodeToString(
			secretKey.Public().ExportBytes(),
		),
		PrivateKeyEncrypted: encryptedPrivate,
		Status:              license.SigningKeyStatusActive,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	key.GenerateID()

	created, err := s.keyRepo.Create(ctx, key)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to persist license signing key")
		return license.SigningKey{}, err
	}

	s.logger.Info().Str("kid", created.ID).Msg("generated new license signing key")
	return created, nil
}

func (s *licenseSigningService) decryptSecretKey(
	key license.SigningKey,
) (paseto.V4AsymmetricSecretKey, error) {
	privateB64, err := s.cipher.DecryptString(key.PrivateKeyEncrypted)
	if err != nil {
		return paseto.V4AsymmetricSecretKey{}, fmt.Errorf(
			"failed to decrypt license signing key %s: %w", key.ID, err,
		)
	}

	privateBytes, err := base64.StdEncoding.DecodeString(privateB64)
	if err != nil {
		return paseto.V4AsymmetricSecretKey{}, fmt.Errorf(
			"failed to decode license signing key %s: %w", key.ID, err,
		)
	}

	return paseto.NewV4AsymmetricSecretKeyFromBytes(privateBytes)
}

// signClaims builds and signs the PASETO v4.public token for the given claims
// with the kid carried in the token footer as JSON.
func signClaims(
	claims license.Claims, secretKey paseto.V4AsymmetricSecretKey, kid string,
) (string, error) {
	token := paseto.NewToken()
	token.SetIssuedAt(claims.IssuedAt)
	token.SetExpiration(claims.ExpiresAt)
	token.SetString(license.ClaimOrganizationID, claims.OrganizationID)
	token.SetString(license.ClaimProductID, claims.ProductID)
	token.SetString(license.ClaimPlanKey, claims.PlanKey)
	token.SetString(license.ClaimStatus, string(claims.Status))

	entitlements := claims.Entitlements
	if entitlements == nil {
		entitlements = plan.Entitlements{}
	}
	if err := token.Set(license.ClaimEntitlements, entitlements); err != nil {
		return "", fmt.Errorf("failed to set entitlements claim: %w", err)
	}

	if claims.GraceUntil != nil {
		token.SetTime(license.ClaimGraceUntil, *claims.GraceUntil)
	}
	token.SetTime(license.ClaimRefreshAfter, claims.RefreshAfter)
	if err := token.Set(license.ClaimSchemaVersion, claims.SchemaVersion); err != nil {
		return "", fmt.Errorf("failed to set schema_version claim: %w", err)
	}

	footer, err := json.Marshal(map[string]string{license.FooterKid: kid})
	if err != nil {
		return "", fmt.Errorf("failed to encode token footer: %w", err)
	}
	token.SetFooter(footer)

	return token.V4Sign(secretKey, nil), nil
}

package service_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	paseto "aidanwoods.dev/go-paseto"
	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/license"
	"anchor/internal/domain/plan"
	"anchor/internal/security/encryption"
	"anchor/internal/service"
	"anchor/internal/service/config"
)

func testClaims(now time.Time) license.Claims {
	grace := now.Add(72 * time.Hour)

	return license.Claims{
		OrganizationID: "org_test123",
		ProductID:      "prd_test123",
		PlanKey:        "pro",
		Status:         license.TokenStatusActive,
		Entitlements: plan.Entitlements{
			"api_access": {Type: plan.EntitlementTypeBoolean, Value: true},
			"max_runs":   {Type: plan.EntitlementTypeNumeric, Value: float64(25)},
		},
		IssuedAt:      now,
		ExpiresAt:     now.Add(24 * time.Hour),
		GraceUntil:    &grace,
		RefreshAfter:  now.Add(12 * time.Hour),
		SchemaVersion: license.ClaimsSchemaVersion,
	}
}

func TestSignClaimsRoundTrip(t *testing.T) {
	t.Parallel()

	secretKey := paseto.NewV4AsymmetricSecretKey()
	now := time.Now().Truncate(time.Second)
	claims := testClaims(now)

	token, err := service.SignClaims(claims, secretKey, "lsk_testkid")
	require.NoError(t, err)
	assert.Contains(t, token, "v4.public.")

	parsed, err := paseto.NewParser().ParseV4Public(secretKey.Public(), token, nil)
	require.NoError(t, err)

	orgID, err := parsed.GetString(license.ClaimOrganizationID)
	require.NoError(t, err)
	assert.Equal(t, claims.OrganizationID, orgID)

	productID, err := parsed.GetString(license.ClaimProductID)
	require.NoError(t, err)
	assert.Equal(t, claims.ProductID, productID)

	planKey, err := parsed.GetString(license.ClaimPlanKey)
	require.NoError(t, err)
	assert.Equal(t, claims.PlanKey, planKey)

	status, err := parsed.GetString(license.ClaimStatus)
	require.NoError(t, err)
	assert.Equal(t, string(license.TokenStatusActive), status)

	var entitlements plan.Entitlements
	require.NoError(t, parsed.Get(license.ClaimEntitlements, &entitlements))
	assert.Equal(t, claims.Entitlements, entitlements)

	expiration, err := parsed.GetExpiration()
	require.NoError(t, err)
	assert.WithinDuration(t, claims.ExpiresAt, expiration, time.Second)

	issuedAt, err := parsed.GetIssuedAt()
	require.NoError(t, err)
	assert.WithinDuration(t, claims.IssuedAt, issuedAt, time.Second)

	refreshAfter, err := parsed.GetTime(license.ClaimRefreshAfter)
	require.NoError(t, err)
	assert.WithinDuration(t, claims.RefreshAfter, refreshAfter, time.Second)

	graceUntil, err := parsed.GetTime(license.ClaimGraceUntil)
	require.NoError(t, err)
	assert.WithinDuration(t, *claims.GraceUntil, graceUntil, time.Second)

	var schemaVersion int
	require.NoError(t, parsed.Get(license.ClaimSchemaVersion, &schemaVersion))
	assert.Equal(t, license.ClaimsSchemaVersion, schemaVersion)

	var footer map[string]string
	require.NoError(t, json.Unmarshal(parsed.Footer(), &footer))
	assert.Equal(t, "lsk_testkid", footer[license.FooterKid])
}

func TestSignClaimsRejectsWrongKey(t *testing.T) {
	t.Parallel()

	secretKey := paseto.NewV4AsymmetricSecretKey()
	otherKey := paseto.NewV4AsymmetricSecretKey()

	token, err := service.SignClaims(testClaims(time.Now()), secretKey, "lsk_testkid")
	require.NoError(t, err)

	_, err = paseto.NewParser().ParseV4Public(otherKey.Public(), token, nil)
	require.Error(t, err)
}

// fakeSigningKeyRepo is an in-memory LicenseSigningKeyRepository.
type fakeSigningKeyRepo struct {
	keys []license.SigningKey
}

func (f *fakeSigningKeyRepo) FindActive(context.Context) (*license.SigningKey, error) {
	// Absent is modelled as a nil result with no error, matching the
	// optional-result contract of the real repository.
	var found *license.SigningKey
	for i := range f.keys {
		if f.keys[i].Status == license.SigningKeyStatusActive {
			key := f.keys[i]
			found = &key
			break
		}
	}
	return found, nil
}

func (f *fakeSigningKeyRepo) ListAll(context.Context) ([]license.SigningKey, error) {
	return f.keys, nil
}

func (f *fakeSigningKeyRepo) Create(
	_ context.Context, key license.SigningKey,
) (license.SigningKey, error) {
	f.keys = append(f.keys, key)
	return key, nil
}

func newTestEncryptionService(t *testing.T) *encryption.Service {
	t.Helper()

	rawKey := make([]byte, 32)
	_, err := rand.Read(rawKey)
	require.NoError(t, err)

	svc, err := encryption.NewService(&config.CoreConfig{
		Encryption: config.EncryptionConfig{
			GlobalKey: base64.StdEncoding.EncodeToString(rawKey),
		},
	})
	require.NoError(t, err)

	return svc
}

func TestLicenseSigningServiceEnsureAndSign(t *testing.T) {
	t.Parallel()

	repo := &fakeSigningKeyRepo{}
	signer := service.NewLicenseSigningService(
		repo, newTestEncryptionService(t), zerolog.Nop(),
	)
	ctx := context.Background()

	// No key yet: signing fails with the typed error.
	_, err := signer.Sign(ctx, testClaims(time.Now()))
	require.ErrorIs(t, err, service.ErrLicenseSigningKeyMissing)

	// Ensure generates one ACTIVE key.
	created, err := signer.EnsureActiveSigningKey(ctx)
	require.NoError(t, err)
	assert.Equal(t, license.SigningKeyStatusActive, created.Status)
	assert.NotEmpty(t, created.ID)

	publicBytes, err := base64.StdEncoding.DecodeString(created.PublicKey)
	require.NoError(t, err)
	assert.Len(t, publicBytes, 32)
	// The private key is stored as VersionedCipher ciphertext, never as plain
	// base64 key material (a bare 64-byte Ed25519 key would decode cleanly).
	assert.NotEmpty(t, created.PrivateKeyEncrypted)
	assert.True(
		t, secrets.IsVersionedEncryptedSecret(created.PrivateKeyEncrypted),
		"private key must be stored as versioned ciphertext",
	)

	// Ensure is idempotent.
	again, err := signer.EnsureActiveSigningKey(ctx)
	require.NoError(t, err)
	assert.Equal(t, created.ID, again.ID)
	assert.Len(t, repo.keys, 1)

	// Sign uses the stored (encrypted) key; the token verifies against the
	// published public key.
	claims := testClaims(time.Now())
	token, err := signer.Sign(ctx, claims)
	require.NoError(t, err)

	publicKey, err := paseto.NewV4AsymmetricPublicKeyFromBytes(publicBytes)
	require.NoError(t, err)

	parsed, err := paseto.NewParser().ParseV4Public(publicKey, token, nil)
	require.NoError(t, err)

	var footer map[string]string
	require.NoError(t, json.Unmarshal(parsed.Footer(), &footer))
	assert.Equal(t, created.ID, footer[license.FooterKid])
}

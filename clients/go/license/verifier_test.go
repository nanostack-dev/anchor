package license_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	paseto "aidanwoods.dev/go-paseto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nanostack-dev/anchor/clients/go/license"
)

const testKid = "lsk_2iTestKid00000000000000000"

type tokenSpec struct {
	status     string
	issuedAt   time.Time
	expiresAt  time.Time
	graceUntil *time.Time
}

// signTestToken mirrors Anchor's token layout: claims payload + kid footer.
func signTestToken(t *testing.T, secretKey paseto.V4AsymmetricSecretKey, spec tokenSpec) string {
	t.Helper()

	token := paseto.NewToken()
	token.SetIssuedAt(spec.issuedAt)
	token.SetExpiration(spec.expiresAt)
	token.SetString("organization_id", "org_test123")
	token.SetString("product_id", "prd_test123")
	token.SetString("plan_key", "pro")
	token.SetString("status", spec.status)
	require.NoError(t, token.Set("entitlements", map[string]license.EntitlementValue{
		"api_access":  {Type: "boolean", Value: true},
		"beta_access": {Type: "boolean", Value: false},
		"max_runs":    {Type: "numeric", Value: float64(25)},
	}))
	if spec.graceUntil != nil {
		token.SetTime("grace_until", *spec.graceUntil)
	}
	token.SetTime("refresh_after", spec.issuedAt.Add(spec.expiresAt.Sub(spec.issuedAt)/2))
	require.NoError(t, token.Set("schema_version", 1))

	footer, err := json.Marshal(map[string]string{"kid": testKid})
	require.NoError(t, err)
	token.SetFooter(footer)

	return token.V4Sign(secretKey, nil)
}

func newTestVerifier(t *testing.T, secretKey paseto.V4AsymmetricSecretKey) *license.Verifier {
	t.Helper()

	verifier, err := license.NewVerifier([]license.PublicKey{{
		Kid: testKid,
		Key: base64.StdEncoding.EncodeToString(secretKey.Public().ExportBytes()),
	}})
	require.NoError(t, err)

	return verifier
}

func activeSpec() tokenSpec {
	now := time.Now()
	return tokenSpec{
		status:    "ACTIVE",
		issuedAt:  now,
		expiresAt: now.Add(24 * time.Hour),
	}
}

func TestNewVerifierValidation(t *testing.T) {
	t.Parallel()

	_, err := license.NewVerifier(nil)
	require.Error(t, err)

	key := base64.StdEncoding.EncodeToString(
		paseto.NewV4AsymmetricSecretKey().Public().ExportBytes(),
	)

	_, err = license.NewVerifier([]license.PublicKey{{Kid: "", Key: key}})
	require.Error(t, err)

	_, err = license.NewVerifier([]license.PublicKey{
		{Kid: "lsk_a", Key: key}, {Kid: "lsk_a", Key: key},
	})
	require.ErrorContains(t, err, "duplicate kid")

	_, err = license.NewVerifier([]license.PublicKey{{Kid: "lsk_a", Key: "not-base64!"}})
	require.Error(t, err)

	_, err = license.NewVerifier([]license.PublicKey{{
		Kid: "lsk_a", Key: base64.StdEncoding.EncodeToString([]byte("short")),
	}})
	require.Error(t, err)
}

func TestVerifyValidToken(t *testing.T) {
	t.Parallel()

	secretKey := paseto.NewV4AsymmetricSecretKey()
	verifier := newTestVerifier(t, secretKey)
	token := signTestToken(t, secretKey, activeSpec())

	claims, status, err := verifier.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, license.StatusValid, status)
	require.NotNil(t, claims)

	assert.Equal(t, "org_test123", claims.OrganizationID)
	assert.Equal(t, "prd_test123", claims.ProductID)
	assert.Equal(t, "pro", claims.PlanKey)
	assert.Equal(t, "ACTIVE", claims.Status)
	assert.Equal(t, 1, claims.SchemaVersion)
	assert.Nil(t, claims.GraceUntil)

	assert.True(t, claims.HasFeature("api_access"))
	assert.False(t, claims.HasFeature("beta_access"), "false-valued feature is not granted")
	assert.False(t, claims.HasFeature("missing"))
	assert.False(t, claims.HasFeature("max_runs"), "numeric entitlement is not a feature")

	limit, ok := claims.Limit("max_runs")
	require.True(t, ok)
	assert.InDelta(t, float64(25), limit, 0.0001)

	_, ok = claims.Limit("api_access")
	assert.False(t, ok, "boolean entitlement is not a limit")
	_, ok = claims.Limit("missing")
	assert.False(t, ok)
}

func TestVerifyTamperedToken(t *testing.T) {
	t.Parallel()

	secretKey := paseto.NewV4AsymmetricSecretKey()
	verifier := newTestVerifier(t, secretKey)
	token := signTestToken(t, secretKey, activeSpec())

	// Flip one character inside the signed payload.
	parts := strings.Split(token, ".")
	payload := []byte(parts[2])
	if payload[10] == 'A' {
		payload[10] = 'B'
	} else {
		payload[10] = 'A'
	}
	parts[2] = string(payload)
	tampered := strings.Join(parts, ".")

	claims, status, err := verifier.Verify(tampered)
	require.Error(t, err)
	assert.Equal(t, license.StatusInvalid, status)
	assert.Nil(t, claims, "claims must be unreachable without a valid signature")
}

func TestVerifyWrongKey(t *testing.T) {
	t.Parallel()

	signingKey := paseto.NewV4AsymmetricSecretKey()
	otherKey := paseto.NewV4AsymmetricSecretKey()
	// Verifier trusts a different key under the same kid.
	verifier := newTestVerifier(t, otherKey)
	token := signTestToken(t, signingKey, activeSpec())

	claims, status, err := verifier.Verify(token)
	require.Error(t, err)
	assert.Equal(t, license.StatusInvalid, status)
	assert.Nil(t, claims)
}

func TestVerifyUnknownKid(t *testing.T) {
	t.Parallel()

	secretKey := paseto.NewV4AsymmetricSecretKey()
	verifier, err := license.NewVerifier([]license.PublicKey{{
		Kid: "lsk_someOtherKid",
		Key: base64.StdEncoding.EncodeToString(secretKey.Public().ExportBytes()),
	}})
	require.NoError(t, err)

	token := signTestToken(t, secretKey, activeSpec())

	claims, status, err := verifier.Verify(token)
	require.ErrorContains(t, err, "unknown kid")
	assert.Equal(t, license.StatusInvalid, status)
	assert.Nil(t, claims)
}

func TestVerifyRejectsNonV4Public(t *testing.T) {
	t.Parallel()

	secretKey := paseto.NewV4AsymmetricSecretKey()
	verifier := newTestVerifier(t, secretKey)

	for _, token := range []string{
		"v4.local.abcdef.footer",
		"v2.public.abcdef",
		"not-a-token",
		"",
	} {
		claims, status, err := verifier.Verify(token)
		require.Error(t, err)
		assert.Equal(t, license.StatusInvalid, status)
		assert.Nil(t, claims)
	}
}

func TestVerifyExpiredToken(t *testing.T) {
	t.Parallel()

	secretKey := paseto.NewV4AsymmetricSecretKey()
	verifier := newTestVerifier(t, secretKey)

	now := time.Now()
	token := signTestToken(t, secretKey, tokenSpec{
		status:    "ACTIVE",
		issuedAt:  now.Add(-48 * time.Hour),
		expiresAt: now.Add(-24 * time.Hour),
	})

	claims, status, err := verifier.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, license.StatusExpired, status)
	require.NotNil(t, claims, "verified-but-expired claims stay readable")
	assert.Equal(t, "org_test123", claims.OrganizationID)
}

func TestVerifyExpiredWithinBusinessGrace(t *testing.T) {
	t.Parallel()

	secretKey := paseto.NewV4AsymmetricSecretKey()
	verifier := newTestVerifier(t, secretKey)

	now := time.Now()
	graceUntil := now.Add(48 * time.Hour)
	token := signTestToken(t, secretKey, tokenSpec{
		status:     "GRACE",
		issuedAt:   now.Add(-48 * time.Hour),
		expiresAt:  now.Add(-1 * time.Hour),
		graceUntil: &graceUntil,
	})

	claims, status, err := verifier.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, license.StatusGrace, status)
	require.NotNil(t, claims)
	require.NotNil(t, claims.GraceUntil)
}

func TestVerifyWithGraceSyncWindow(t *testing.T) {
	t.Parallel()

	secretKey := paseto.NewV4AsymmetricSecretKey()
	verifier := newTestVerifier(t, secretKey)

	now := time.Now()
	token := signTestToken(t, secretKey, tokenSpec{
		status:    "ACTIVE",
		issuedAt:  now.Add(-25 * time.Hour),
		expiresAt: now.Add(-1 * time.Hour),
	})

	// Strict verify: expired.
	_, status, err := verifier.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, license.StatusExpired, status)

	// Within the sync grace window: GRACE.
	claims, status, err := verifier.VerifyWithGrace(token, 2*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, license.StatusGrace, status)
	require.NotNil(t, claims)

	// Sync grace smaller than the overshoot: still expired.
	_, status, err = verifier.VerifyWithGrace(token, 30*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, license.StatusExpired, status)
}

func TestVerifySuspendedToken(t *testing.T) {
	t.Parallel()

	secretKey := paseto.NewV4AsymmetricSecretKey()
	verifier := newTestVerifier(t, secretKey)
	spec := activeSpec()
	spec.status = "SUSPENDED"
	token := signTestToken(t, secretKey, spec)

	claims, status, err := verifier.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, license.StatusSuspended, status)
	require.NotNil(t, claims)
	assert.Equal(t, "SUSPENDED", claims.Status)
}

func TestVerifyEmbeddedGraceStatus(t *testing.T) {
	t.Parallel()

	secretKey := paseto.NewV4AsymmetricSecretKey()
	verifier := newTestVerifier(t, secretKey)
	spec := activeSpec()
	spec.status = "GRACE"
	token := signTestToken(t, secretKey, spec)

	_, status, err := verifier.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, license.StatusGrace, status)
}

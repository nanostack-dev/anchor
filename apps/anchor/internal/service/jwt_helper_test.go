package service_test

import (
	"testing"
	"time"

	"anchor/internal/service"
	"anchor/internal/service/config"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSubject(userID, tenantID string) service.TokenSubject {
	return service.TokenSubject{
		UserID:         userID,
		TenantID:       tenantID,
		SessionID:      "psess_test",
		RefreshTokenID: "prtok_test",
	}
}

func createTestAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		AdminJWTSecret:       "test-secret-key-for-testing-purposes-only",
		AccessTokenLifetime:  3600,  // 1 hour
		RefreshTokenLifetime: 86400, // 24 hours
	}
}

func TestJWTHelper_AudienceValidation(t *testing.T) {
	authCfg := createTestAuthConfig()
	jwtHelper := service.NewJWTHelper(authCfg)

	userID := "user_test123"
	tenantID := "tenant_test456"

	issued, err := jwtHelper.GenerateTokens(testSubject(userID, tenantID))
	require.NoError(t, err)
	accessToken, refreshToken := issued.AccessToken, issued.RefreshToken
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)

	t.Run(
		"ValidateAccessToken with access token should succeed", func(t *testing.T) {
			claims, validateErr := jwtHelper.ValidateAccessToken(accessToken)
			require.NoError(t, validateErr)
			assert.Equal(t, userID, claims.UserID)
			assert.Equal(t, tenantID, claims.TenantID)
			assert.Contains(t, claims.Audience, "anchor_access")
		},
	)

	t.Run(
		"ValidateRefreshToken with refresh token should succeed", func(t *testing.T) {
			claims, validateErr := jwtHelper.ValidateRefreshToken(refreshToken)
			require.NoError(t, validateErr)
			assert.Equal(t, userID, claims.UserID)
			assert.Equal(t, tenantID, claims.TenantID)
			assert.Contains(t, claims.Audience, "anchor_refresh")
		},
	)

	t.Run(
		"ValidateAccessToken with refresh token should fail", func(t *testing.T) {
			_, validateErr := jwtHelper.ValidateAccessToken(refreshToken)
			require.Error(t, validateErr)
			assert.Contains(t, validateErr.Error(), "token has invalid audience")
			assert.Contains(t, validateErr.Error(), "expected anchor_access")
		},
	)

	t.Run(
		"ValidateRefreshToken with access token should fail", func(t *testing.T) {
			_, validateErr := jwtHelper.ValidateRefreshToken(accessToken)
			require.Error(t, validateErr)
			assert.Contains(t, validateErr.Error(), "token has invalid audience")
			assert.Contains(t, validateErr.Error(), "expected anchor_refresh")
		},
	)

	t.Run(
		"Malformed token should be rejected", func(t *testing.T) {
			malformedToken := "invalid.jwt.token"

			_, accessErr := jwtHelper.ValidateAccessToken(malformedToken)
			require.Error(t, accessErr)
			assert.Contains(t, accessErr.Error(), "token validation failed")

			_, refreshErr := jwtHelper.ValidateRefreshToken(malformedToken)
			require.Error(t, refreshErr)
			assert.Contains(t, refreshErr.Error(), "token validation failed")
		},
	)
}

func TestJWTHelper_TokenGeneration(t *testing.T) {
	authCfg := createTestAuthConfig()
	jwtHelper := service.NewJWTHelper(authCfg)

	userID := "user_test789"
	tenantID := "tenant_test101"

	t.Run(
		"GenerateTokens should create tokens with correct audiences", func(t *testing.T) {
			issued, err := jwtHelper.GenerateTokens(testSubject(userID, tenantID))
			require.NoError(t, err)
			accessToken, refreshToken := issued.AccessToken, issued.RefreshToken
			require.NotEmpty(t, accessToken)
			require.NotEmpty(t, refreshToken)

			// Parse and verify access token audience
			accessClaims := &service.AuthClaims{}
			_, err = jwtlib.ParseWithClaims(
				accessToken, accessClaims, func(_ *jwtlib.Token) (any, error) {
					return authCfg.GetAdminJWTSecretAsBytes(), nil
				},
			)
			require.NoError(t, err)
			assert.Contains(t, accessClaims.Audience, "anchor_access")
			assert.Equal(t, userID, accessClaims.UserID)
			assert.Equal(t, tenantID, accessClaims.TenantID)
			assert.Equal(t, "psess_test", accessClaims.SessionID)

			// Parse and verify refresh token audience
			refreshClaims := &service.AuthClaims{}
			_, err = jwtlib.ParseWithClaims(
				refreshToken, refreshClaims, func(_ *jwtlib.Token) (any, error) {
					return authCfg.GetAdminJWTSecretAsBytes(), nil
				},
			)
			require.NoError(t, err)
			assert.Contains(t, refreshClaims.Audience, "anchor_refresh")
			assert.Equal(t, userID, refreshClaims.UserID)
			assert.Equal(t, tenantID, refreshClaims.TenantID)
			assert.Equal(t, "psess_test", refreshClaims.SessionID)
			assert.Equal(t, "prtok_test", refreshClaims.ID)
			assert.WithinDuration(t, issued.RefreshExpiresAt, refreshClaims.ExpiresAt.Time, time.Second)
		},
	)

	t.Run(
		"GenerateTokens should validate input parameters", func(t *testing.T) {
			_, err := jwtHelper.GenerateTokens(testSubject("invalid_user", tenantID))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "userID and tenantID cannot be empty or are not valid")

			_, err = jwtHelper.GenerateTokens(testSubject(userID, "invalid_tenant"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "userID and tenantID cannot be empty or are not valid")

			withoutSession := testSubject(userID, tenantID)
			withoutSession.SessionID = ""
			_, err = jwtHelper.GenerateTokens(withoutSession)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "sessionID and refreshTokenID cannot be empty")
		},
	)
}

func TestJWTHelper_VerifyAccessTokenIgnoringExpiry(t *testing.T) {
	authCfg := createTestAuthConfig()
	authCfg.AccessTokenLifetime = -60
	expiredIssued, err := service.NewJWTHelper(authCfg).GenerateTokens(testSubject("user_test1", "tenant_test1"))
	require.NoError(t, err)
	jwtHelper := service.NewJWTHelper(createTestAuthConfig())

	t.Run("ExpiredAccessTokenIsRefusedForAuthorization", func(t *testing.T) {
		_, validateErr := jwtHelper.ValidateAccessToken(expiredIssued.AccessToken)
		require.Error(t, validateErr)
	})

	t.Run("ExpiredAccessTokenStillNamesItsSession", func(t *testing.T) {
		claims, verifyErr := jwtHelper.VerifyAccessTokenIgnoringExpiry(expiredIssued.AccessToken)
		require.NoError(t, verifyErr)
		assert.Equal(t, "psess_test", claims.SessionID)
	})

	t.Run("RefreshTokenIsRefused", func(t *testing.T) {
		_, verifyErr := jwtHelper.VerifyAccessTokenIgnoringExpiry(expiredIssued.RefreshToken)
		require.Error(t, verifyErr)
	})

	t.Run("TokenSignedWithAnotherSecretIsRefused", func(t *testing.T) {
		otherCfg := createTestAuthConfig()
		otherCfg.AdminJWTSecret = "another-secret-key-for-testing-purposes"
		forged, forgeErr := service.NewJWTHelper(otherCfg).GenerateTokens(testSubject("user_test1", "tenant_test1"))
		require.NoError(t, forgeErr)
		_, verifyErr := jwtHelper.VerifyAccessTokenIgnoringExpiry(forged.AccessToken)
		require.Error(t, verifyErr)
	})
}

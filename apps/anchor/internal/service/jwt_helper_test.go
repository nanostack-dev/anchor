package service_test

import (
	"testing"

	"anchor/internal/service"
	"anchor/internal/service/config"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	// Generate tokens
	accessToken, refreshToken, err := jwtHelper.GenerateTokens(userID, tenantID)
	require.NoError(t, err)
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
			accessToken, refreshToken, err := jwtHelper.GenerateTokens(userID, tenantID)
			require.NoError(t, err)
			require.NotEmpty(t, accessToken)
			require.NotEmpty(t, refreshToken)

			// Parse and verify access token audience
			accessClaims := &service.AuthClaims{}
			_, err = jwtlib.ParseWithClaims(
				accessToken, accessClaims, func(_ *jwtlib.Token) (interface{}, error) {
					return authCfg.GetAdminJWTSecretAsBytes(), nil
				},
			)
			require.NoError(t, err)
			assert.Contains(t, accessClaims.Audience, "anchor_access")
			assert.Equal(t, userID, accessClaims.UserID)
			assert.Equal(t, tenantID, accessClaims.TenantID)

			// Parse and verify refresh token audience
			refreshClaims := &service.AuthClaims{}
			_, err = jwtlib.ParseWithClaims(
				refreshToken, refreshClaims, func(_ *jwtlib.Token) (interface{}, error) {
					return authCfg.GetAdminJWTSecretAsBytes(), nil
				},
			)
			require.NoError(t, err)
			assert.Contains(t, refreshClaims.Audience, "anchor_refresh")
			assert.Equal(t, userID, refreshClaims.UserID)
			assert.Equal(t, tenantID, refreshClaims.TenantID)
		},
	)

	t.Run(
		"GenerateTokens should validate input parameters", func(t *testing.T) {
			_, _, err := jwtHelper.GenerateTokens("invalid_user", tenantID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "userID and tenantID cannot be empty or are not valid")

			_, _, err = jwtHelper.GenerateTokens(userID, "invalid_tenant")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "userID and tenantID cannot be empty or are not valid")
		},
	)
}

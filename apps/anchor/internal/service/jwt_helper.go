package service

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"anchor/internal/domain/session"
	"anchor/internal/service/config"

	"github.com/golang-jwt/jwt/v5"
)

type AuthClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
	TenantID  string           `json:"tenant_id"`
	SessionID string           `json:"sid,omitempty"`
	AuthTime  *jwt.NumericDate `json:"auth_time,omitempty"`
}

type JWTHelper interface {
	// GenerateTokens signs an access token for the session and the refresh
	// token whose row the caller stores: its id is the jti, its expiry the exp.
	// authTime is when the session's user last signed in with a password.
	GenerateTokens(userID, tenantID string, authTime time.Time, refreshToken session.RefreshToken) (
		accessToken string, signedRefreshToken string, err error,
	)
	ValidateAccessToken(tokenString string) (*AuthClaims, error)
	// ValidateAccessTokenIgnoringExpiry checks an access token's signature and
	// audience but not its expiry, for logout, which must work after the
	// access token has lapsed. Never use it to authorize a request.
	ValidateAccessTokenIgnoringExpiry(tokenString string) (*AuthClaims, error)
	ValidateRefreshToken(tokenString string) (*AuthClaims, error)
}

type jwtHelper struct {
	authCfg config.AuthConfig
}

func NewJWTHelper(authCfg config.AuthConfig) JWTHelper {
	return &jwtHelper{
		authCfg: authCfg,
	}
}

func (h *jwtHelper) GenerateTokens(
	userID, tenantID string, authTime time.Time, refreshToken session.RefreshToken,
) (string, string, error) {
	if !strings.HasPrefix(userID, "user_") || !strings.HasPrefix(tenantID, "tenant_") {
		return "", "", errors.New("userID and tenantID cannot be empty or are not valid")
	}

	now := time.Now()
	accessClaims := AuthClaims{
		UserID:    userID,
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Second * time.Duration(h.authCfg.AccessTokenLifetime))),
		IssuedAt:  jwt.NewNumericDate(now),
		Subject:   userID,
		Issuer:    "anchor",
		Audience:  jwt.ClaimStrings{"anchor_access"},
		TenantID:  tenantID,
		SessionID: refreshToken.SessionID,
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).
		SignedString(h.authCfg.GetAdminJWTSecretAsBytes())
	if err != nil {
		return "", "", fmt.Errorf("failed to sign access token: %w", err)
	}

	refreshClaims := AuthClaims{
		UserID:    userID,
		ID:        refreshToken.ID,
		ExpiresAt: jwt.NewNumericDate(refreshToken.ExpiresAt),
		IssuedAt:  jwt.NewNumericDate(now),
		Subject:   userID,
		Issuer:    "anchor",
		Audience:  jwt.ClaimStrings{"anchor_refresh"},
		TenantID:  tenantID,
		SessionID: refreshToken.SessionID,
		AuthTime:  jwt.NewNumericDate(authTime),
	}
	signedRefreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).
		SignedString(h.authCfg.GetAdminJWTSecretAsBytes())
	if err != nil {
		return "", "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return accessToken, signedRefreshToken, nil
}

func (h *jwtHelper) ValidateAccessToken(tokenString string) (*AuthClaims, error) {
	return h.validateTokenWithAudience(tokenString, "anchor_access")
}

func (h *jwtHelper) ValidateAccessTokenIgnoringExpiry(tokenString string) (*AuthClaims, error) {
	return h.validateTokenWithAudience(tokenString, "anchor_access", jwt.WithoutClaimsValidation())
}

func (h *jwtHelper) ValidateRefreshToken(tokenString string) (*AuthClaims, error) {
	return h.validateTokenWithAudience(tokenString, "anchor_refresh")
}

func (h *jwtHelper) validateTokenWithAudience(
	tokenString string, expectedAudience string, parserOptions ...jwt.ParserOption,
) (*AuthClaims, error) {
	claims := &AuthClaims{}

	token, err := jwt.ParseWithClaims(
		tokenString, claims, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return h.authCfg.GetAdminJWTSecretAsBytes(), nil
		},
		parserOptions...,
	)

	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("token is invalid")
	}

	// Validate audience if specified
	if expectedAudience != "" {
		audienceValid := slices.Contains(claims.Audience, expectedAudience)
		if !audienceValid {
			return nil, fmt.Errorf(
				"token has invalid audience: expected %s, got %v", expectedAudience,
				claims.Audience,
			)
		}
	}

	return claims, nil
}

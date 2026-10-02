package service

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"anchor/internal/service/config"

	"github.com/golang-jwt/jwt/v5"
)

type AuthClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
	TenantID  string `json:"tenant_id"`
	SessionID string `json:"sid,omitempty"`
}

// TokenSubject names who a token pair is for: the user and tenant every
// endpoint authorizes against, the session logout ends, and the refresh
// token's own row, carried as its jti.
type TokenSubject struct {
	UserID         string
	TenantID       string
	SessionID      string
	RefreshTokenID string
}

type IssuedTokens struct {
	AccessToken      string
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type JWTHelper interface {
	GenerateTokens(subject TokenSubject) (IssuedTokens, error)
	ValidateAccessToken(tokenString string) (*AuthClaims, error)
	// VerifyAccessTokenIgnoringExpiry checks an access token's signature and
	// audience but not its expiry, for logout, which must work after the
	// access token has lapsed. Never use it to authorize a request.
	VerifyAccessTokenIgnoringExpiry(tokenString string) (*AuthClaims, error)
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

func (h *jwtHelper) GenerateTokens(subject TokenSubject) (IssuedTokens, error) {
	if !strings.HasPrefix(subject.UserID, "user_") || !strings.HasPrefix(subject.TenantID, "tenant_") {
		return IssuedTokens{}, errors.New("userID and tenantID cannot be empty or are not valid")
	}
	if subject.SessionID == "" || subject.RefreshTokenID == "" {
		return IssuedTokens{}, errors.New("sessionID and refreshTokenID cannot be empty")
	}

	now := time.Now()
	accessExpireTime := now.Add(time.Second * time.Duration(h.authCfg.AccessTokenLifetime))
	accessClaims := AuthClaims{
		UserID:    subject.UserID,
		ExpiresAt: jwt.NewNumericDate(accessExpireTime),
		IssuedAt:  jwt.NewNumericDate(now),
		Subject:   subject.UserID,
		Issuer:    "anchor",
		Audience:  jwt.ClaimStrings{"anchor_access"},
		TenantID:  subject.TenantID,
		SessionID: subject.SessionID,
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).
		SignedString(h.authCfg.GetAdminJWTSecretAsBytes())
	if err != nil {
		return IssuedTokens{}, fmt.Errorf("failed to sign access token: %w", err)
	}

	refreshExpireTime := now.Add(time.Second * time.Duration(h.authCfg.RefreshTokenLifetime))
	refreshClaims := AuthClaims{
		UserID:    subject.UserID,
		ID:        subject.RefreshTokenID,
		ExpiresAt: jwt.NewNumericDate(refreshExpireTime),
		IssuedAt:  jwt.NewNumericDate(now),
		Subject:   subject.UserID,
		Issuer:    "anchor",
		Audience:  jwt.ClaimStrings{"anchor_refresh"},
		TenantID:  subject.TenantID,
		SessionID: subject.SessionID,
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).
		SignedString(h.authCfg.GetAdminJWTSecretAsBytes())
	if err != nil {
		return IssuedTokens{}, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return IssuedTokens{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshExpireTime,
	}, nil
}

func (h *jwtHelper) ValidateAccessToken(tokenString string) (*AuthClaims, error) {
	return h.validateTokenWithAudience(tokenString, "anchor_access")
}

func (h *jwtHelper) VerifyAccessTokenIgnoringExpiry(tokenString string) (*AuthClaims, error) {
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

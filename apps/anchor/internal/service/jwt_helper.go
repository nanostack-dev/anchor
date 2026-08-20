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
	TenantID string `json:"tenant_id"`
}

type JWTHelper interface {
	GenerateTokens(userID string, tenantID string) (
		accessToken string, refreshToken string,
		err error,
	)
	ValidateAccessToken(tokenString string) (*AuthClaims, error)
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

func (h *jwtHelper) GenerateTokens(userID string, tenantID string) (string, string, error) {
	if !strings.HasPrefix(userID, "user_") || !strings.HasPrefix(tenantID, "tenant_") {
		return "", "", errors.New("userID and tenantID cannot be empty or are not valid")
	}

	var accessToken, refreshToken string
	var err error

	accessExpireTime := time.Now().Add(time.Second * time.Duration(h.authCfg.AccessTokenLifetime))
	accessClaims := AuthClaims{
		UserID:    userID,
		ExpiresAt: jwt.NewNumericDate(accessExpireTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   userID,
		Issuer:    "anchor",
		Audience:  jwt.ClaimStrings{"anchor_access"},
		TenantID:  tenantID,
	}
	accessTokenJWT := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = accessTokenJWT.SignedString(h.authCfg.GetAdminJWTSecretAsBytes())
	if err != nil {
		return "", "", fmt.Errorf("failed to sign access token: %w", err)
	}

	refreshExpireTime := time.Now().Add(time.Second * time.Duration(h.authCfg.RefreshTokenLifetime))
	refreshClaims := AuthClaims{

		UserID:    userID,
		ExpiresAt: jwt.NewNumericDate(refreshExpireTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   userID,
		Issuer:    "anchor",
		Audience:  jwt.ClaimStrings{"anchor_refresh"},
		TenantID:  tenantID,
	}
	refreshTokenJWT := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshTokenJWT.SignedString(h.authCfg.GetAdminJWTSecretAsBytes())
	if err != nil {
		return "", "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

func (h *jwtHelper) ValidateAccessToken(tokenString string) (*AuthClaims, error) {
	return h.validateTokenWithAudience(tokenString, "anchor_access")
}

func (h *jwtHelper) ValidateRefreshToken(tokenString string) (*AuthClaims, error) {
	return h.validateTokenWithAudience(tokenString, "anchor_refresh")
}

func (h *jwtHelper) validateTokenWithAudience(
	tokenString string, expectedAudience string,
) (*AuthClaims, error) {
	claims := &AuthClaims{}

	token, err := jwt.ParseWithClaims(
		tokenString, claims, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return h.authCfg.GetAdminJWTSecretAsBytes(), nil
		},
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

package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"anchor/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubJWTHelper struct {
	service.JWTHelper
	validateFn func(tokenString string) (*service.AuthClaims, error)
}

func (s *stubJWTHelper) ValidateAccessToken(tokenString string) (*service.AuthClaims, error) {
	if s.validateFn != nil {
		return s.validateFn(tokenString)
	}
	return nil, errors.New("unimplemented")
}

func TestValidateAccessToken_RequiresBearerPrefix(t *testing.T) {
	t.Parallel()

	auth := &AuthMiddleware{}

	tests := []struct {
		name          string
		authHeader    string
		expectedError string
	}{
		{
			name:          "missing authorization header",
			authHeader:    "",
			expectedError: "invalid bearer format",
		},
		{
			name:          "short authorization header",
			authHeader:    "Bearer",
			expectedError: "invalid bearer format",
		},
		{
			name:          "arbitrary 7-byte prefix",
			authHeader:    "XXXXXXXvalid.jwt.token",
			expectedError: "invalid bearer format",
		},
		{
			name:          "basic auth prefix",
			authHeader:    "Basic  valid.jwt.token",
			expectedError: "invalid bearer format",
		},
		{
			name:          "lowercase bearer prefix",
			authHeader:    "bearer valid.jwt.token",
			expectedError: "invalid bearer format",
		},
		{
			name:          "bearer prefix with empty token",
			authHeader:    "Bearer ",
			expectedError: "invalid bearer format",
		},
		{
			name:          "whitespace only 7-byte prefix",
			authHeader:    "       valid.jwt.token",
			expectedError: "invalid bearer format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			claims, err := auth.validateAccessToken(req)
			assert.Nil(t, claims)
			require.Error(t, err)
			assert.Equal(t, tc.expectedError, err.Error())
		})
	}
}

func TestValidateAccessToken_ExtractsValidBearerToken(t *testing.T) {
	t.Parallel()

	var passedToken string
	stub := &stubJWTHelper{
		validateFn: func(tokenString string) (*service.AuthClaims, error) {
			passedToken = tokenString
			return nil, errors.New("token verification failed")
		},
	}

	auth := &AuthMiddleware{
		jwtHelper: stub,
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer my-secret-jwt-token")

	claims, err := auth.validateAccessToken(req)
	assert.Nil(t, claims)
	require.Error(t, err)
	assert.Equal(t, "access token rejected", err.Error())
	assert.Equal(t, "my-secret-jwt-token", passedToken)
}

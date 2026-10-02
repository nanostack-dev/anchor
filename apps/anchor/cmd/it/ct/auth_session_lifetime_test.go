package ct_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/internal/domain/session"
	"anchor/internal/service"
	sessionservice "anchor/internal/session/service"
)

func seconds(lifetime int64) time.Duration {
	return time.Duration(lifetime) * time.Second
}

func refreshTokenExpiry(t *testing.T, refreshToken string) time.Time {
	t.Helper()
	claims := &service.AuthClaims{}
	_, _, err := jwt.NewParser().ParseUnverified(refreshToken, claims)
	require.NoError(t, err)
	require.NotNil(t, claims.ExpiresAt)
	return claims.ExpiresAt.Time
}

func refreshCookieExpiry(t *testing.T, resp *http.Response) time.Time {
	t.Helper()
	cookie, err := http.ParseSetCookie(resp.Header.Get("Set-Cookie"))
	require.NoError(t, err)
	require.Equal(t, "refresh_token", cookie.Name)
	return cookie.Expires
}

// signedInAt opens a session for the admin as if they had signed in with
// their password at authTime, and returns its refresh token.
func signedInAt(t *testing.T, state *itdsl.State, user *itdsl.PlatformUser, authTime time.Time) string {
	t.Helper()
	firstToken := session.RefreshToken{
		ID:        session.NewRefreshTokenID(),
		SessionID: session.NewID(),
		ExpiresAt: authTime.Add(seconds(authCfg.SessionMaxLifetime)),
	}
	_, refreshToken, err := TokenHelper.GenerateTokens(
		user.UserID, state.Tenant(user.TenantAlias).ID, authTime, firstToken,
	)
	require.NoError(t, err)
	require.NoError(t, SessionSvc.Start(t.Context(), sessionservice.StartInput{
		PlatformUserID: user.ID,
		FirstToken:     firstToken,
	}))
	return refreshToken
}

func TestRefreshExtendsTheSessionByTheIdleLifetime(t *testing.T) {
	t.Parallel()
	_, user := givenSignedInAdmin(t)

	resp := itdsl.Auth(t).RefreshRaw(user.RefreshToken)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON200)

	idleEnd := time.Now().Add(seconds(authCfg.RefreshTokenLifetime))
	assert.WithinDuration(t, idleEnd, refreshTokenExpiry(t, resp.JSON200.RefreshToken), time.Minute)
	assert.WithinDuration(t, idleEnd, refreshCookieExpiry(t, resp.HTTPResponse), time.Minute)
}

func TestRefreshNeverExtendsPastTheSessionMaxLifetime(t *testing.T) {
	t.Parallel()
	state, user := givenSignedInAdmin(t)
	sessionEnd := time.Now().Add(time.Hour)
	refreshToken := signedInAt(t, state, user, sessionEnd.Add(-seconds(authCfg.SessionMaxLifetime)))

	resp := itdsl.Auth(t).RefreshRaw(refreshToken)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON200)

	assert.WithinDuration(t, sessionEnd, refreshTokenExpiry(t, resp.JSON200.RefreshToken), time.Minute)
	assert.WithinDuration(t, sessionEnd, refreshCookieExpiry(t, resp.HTTPResponse), time.Minute)
}

func TestRefreshAfterTheSessionMaxLifetimeIsRefused(t *testing.T) {
	t.Parallel()
	state, user := givenSignedInAdmin(t)
	refreshToken := signedInAt(t, state, user, time.Now().Add(-seconds(authCfg.SessionMaxLifetime)-time.Minute))

	resp := itdsl.Auth(t).RefreshRaw(refreshToken)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
}

package ct_test

import (
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/internal/domain/session"
	"anchor/internal/service"
)

const sessionUserAlias = "admin.session"

func givenSignedInAdmin(t *testing.T) (*itdsl.State, *itdsl.PlatformUser) {
	t.Helper()
	tenantAlias := "tenant.session." + itshared.Faker.UUID().V4()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: tenantAlias}).
		PlatformAdmin(itdsl.PlatformAdminOpts{Alias: sessionUserAlias, TenantAlias: tenantAlias}).
		Build()
	return state, state.PlatformUser(sessionUserAlias)
}

// endRotationGrace moves a rotated refresh token's rotation back past the
// reuse grace, as if the client presented it again much later.
func endRotationGrace(t *testing.T, refreshToken string) {
	t.Helper()
	claims := &service.AuthClaims{}
	_, _, err := jwt.NewParser().ParseUnverified(refreshToken, claims)
	require.NoError(t, err)
	result, err := testDB.ExecContext(t.Context(),
		`UPDATE platform_user_refresh_tokens
		 SET rotated_at = rotated_at - make_interval(secs => $1)
		 WHERE id = $2 AND rotated_at IS NOT NULL`,
		session.ReuseGrace.Seconds()+1, claims.ID,
	)
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected, "the refresh token was never rotated")
}

func TestRefreshAfterLogoutIsRefused(t *testing.T) {
	t.Parallel()
	_, user := givenSignedInAdmin(t)

	itdsl.Auth(t).Logout(user.AccessToken)

	resp := itdsl.Auth(t).RefreshRaw(user.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
	assert.Empty(t, resp.Body)
}

func TestRefreshOfATokenRotatedBeforeLogoutIsRefused(t *testing.T) {
	t.Parallel()
	_, user := givenSignedInAdmin(t)
	rotated := itdsl.Auth(t).Refresh(user.RefreshToken)

	itdsl.Auth(t).Logout(rotated.AccessToken)

	resp := itdsl.Auth(t).RefreshRaw(rotated.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
}

func TestLogoutLeavesTheUsersOtherSessionsSignedIn(t *testing.T) {
	t.Parallel()
	state, user := givenSignedInAdmin(t)
	otherDevice := state.NewSession(sessionUserAlias)

	itdsl.Auth(t).Logout(user.AccessToken)

	itdsl.Auth(t).Refresh(otherDevice.RefreshToken)
}

func TestLogoutWithoutABearerTokenRevokesNothing(t *testing.T) {
	t.Parallel()
	_, user := givenSignedInAdmin(t)

	resp, err := testTenant(t).NoAuthClient.LogoutWithResponse(t.Context())
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

	itdsl.Auth(t).Refresh(user.RefreshToken)
}

func TestLogoutWithAnUnverifiableBearerTokenRevokesNothing(t *testing.T) {
	t.Parallel()
	_, user := givenSignedInAdmin(t)

	itdsl.Auth(t).Logout("not-a-jwt")

	itdsl.Auth(t).Refresh(user.RefreshToken)
}

func TestAccessTokenStaysValidUntilItExpiresAfterLogout(t *testing.T) {
	t.Parallel()
	_, user := givenSignedInAdmin(t)

	itdsl.Auth(t).Logout(user.AccessToken)

	resp, err := user.AuthenticatedClient.GetCurrentUserWithResponse(t.Context())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
}

func TestRefreshRotatesTheRefreshToken(t *testing.T) {
	t.Parallel()
	_, user := givenSignedInAdmin(t)

	resp := itdsl.Auth(t).RefreshRaw(user.RefreshToken)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON200)
	rotated := *resp.JSON200

	assert.NotEqual(t, user.RefreshToken, rotated.RefreshToken)
	require.NotNil(t, resp.Headers200)
	require.NotNil(t, resp.Headers200.SetCookie)
	assert.Contains(t, *resp.Headers200.SetCookie, "refresh_token="+rotated.RefreshToken)
	itdsl.Auth(t).Refresh(rotated.RefreshToken)
}

func TestRotatedRefreshTokenIsAcceptedWithinTheGrace(t *testing.T) {
	t.Parallel()
	_, user := givenSignedInAdmin(t)

	firstTab := itdsl.Auth(t).Refresh(user.RefreshToken)
	secondTab := itdsl.Auth(t).Refresh(user.RefreshToken)

	itdsl.Auth(t).Refresh(firstTab.RefreshToken)
	itdsl.Auth(t).Refresh(secondTab.RefreshToken)
}

func TestRotatedRefreshTokenIsRefusedAfterTheGrace(t *testing.T) {
	t.Parallel()
	_, user := givenSignedInAdmin(t)
	itdsl.Auth(t).Refresh(user.RefreshToken)
	endRotationGrace(t, user.RefreshToken)

	resp := itdsl.Auth(t).RefreshRaw(user.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
}

func TestReplayedRotatedRefreshTokenRevokesTheWholeSession(t *testing.T) {
	t.Parallel()
	state, user := givenSignedInAdmin(t)
	otherDevice := state.NewSession(sessionUserAlias)
	rotated := itdsl.Auth(t).Refresh(user.RefreshToken)
	endRotationGrace(t, user.RefreshToken)

	replay := itdsl.Auth(t).RefreshRaw(user.RefreshToken)
	require.Equal(t, http.StatusUnauthorized, replay.StatusCode(), string(replay.Body))

	resp := itdsl.Auth(t).RefreshRaw(rotated.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
	itdsl.Auth(t).Refresh(otherDevice.RefreshToken)
}

func TestRefreshOfADeletedPlatformUserIsRefused(t *testing.T) {
	t.Parallel()
	state, user := givenSignedInAdmin(t)
	deleteResp, err := state.Tenant(user.TenantAlias).OwnerClient.
		DeletePlatformUserWithResponse(t.Context(), user.ID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, deleteResp.StatusCode(), string(deleteResp.Body))

	resp := itdsl.Auth(t).RefreshRaw(user.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
}

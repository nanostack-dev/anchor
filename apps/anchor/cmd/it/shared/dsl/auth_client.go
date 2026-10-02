package itdsl

import (
	"context"
	"net/http"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	dslfactory "anchor/cmd/it/shared/dsl/factory"
	"anchor/internal/domain/auth"
)

// AuthClient drives the session routes, refresh and logout, each with the
// token the act names rather than a stored credential.
type AuthClient struct {
	t      *testing.T
	client *nanostackClient.ClientWithResponses
}

func Auth(t *testing.T) AuthClient {
	t.Helper()
	return AuthClient{t: t, client: dslfactory.NewNoAuthClient(t, itshared.ServerURL)}
}

func (c AuthClient) RefreshRaw(refreshToken string) *nanostackClient.RefreshTokenResponse {
	c.t.Helper()
	resp, err := c.client.RefreshTokenWithResponse(
		context.Background(), &nanostackClient.RefreshTokenParams{RefreshToken: &refreshToken},
	)
	require.NoError(c.t, err)
	return resp
}

func (c AuthClient) Refresh(refreshToken string) nanostackClient.AuthTokenResponse {
	c.t.Helper()
	resp := c.RefreshRaw(refreshToken)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

// Logout ends the session the access token belongs to.
func (c AuthClient) Logout(accessToken string) {
	c.t.Helper()
	resp, err := dslfactory.NewBearerClient(c.t, itshared.ServerURL, accessToken).
		LogoutWithResponse(context.Background())
	require.NoError(c.t, err)
	require.Equal(c.t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
}

// NewSession signs the platform user in once more, as another device would,
// and returns that session's token pair.
func (s *State) NewSession(userAlias string) auth.LoginOutput {
	s.t.Helper()
	user := s.PlatformUser(userAlias)
	tokens, err := itshared.AuthService.StartSession(
		context.Background(), auth.StartSessionInput{
			PlatformUserID: user.ID,
			UserID:         user.UserID,
			TenantID:       s.Tenant(user.TenantAlias).ID,
		},
	)
	require.NoError(s.t, err)
	return tokens
}

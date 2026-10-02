package itdsl

import (
	"context"
	"net/http"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// PlatformUser returns the platform user context for an alias.
func (s *State) PlatformUser(alias string) *PlatformUser {
	s.t.Helper()

	user, ok := s.users[alias]
	require.True(s.t, ok, "unknown platform user alias '%s'", alias)

	return user
}

// PlatformUserClient returns an authenticated platform user client for an alias.
func (s *State) PlatformUserClient(alias string) *nanostackClient.ClientWithResponses {
	s.t.Helper()

	user := s.PlatformUser(alias)
	require.NotNil(s.t, user.AuthenticatedClient, "platform user '%s' has no client", alias)

	return user.AuthenticatedClient
}

// NewSession signs the platform user in again over the API, as another device
// would.
func (s *State) NewSession(userAlias string) nanostackClient.AuthTokenResponse {
	s.t.Helper()
	user := s.PlatformUser(userAlias)
	resp, err := s.Tenant(user.TenantAlias).NoAuthClient.LoginWithResponse(
		context.Background(),
		nanostackClient.LoginJSONRequestBody{Email: user.Email, Password: user.Password},
	)
	require.NoError(s.t, err)
	require.Equal(s.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(s.t, resp.JSON200)
	return *resp.JSON200
}

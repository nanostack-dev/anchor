package itdsl

import (
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

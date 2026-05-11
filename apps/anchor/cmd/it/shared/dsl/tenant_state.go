package itdsl

import (
	"github.com/stretchr/testify/require"
)

// Tenant returns the tenant context for an alias.
func (s *State) Tenant(alias string) *Tenant {
	s.t.Helper()

	tenant, ok := s.tenants[alias]
	require.True(s.t, ok, "unknown tenant alias '%s'", alias)

	return tenant
}

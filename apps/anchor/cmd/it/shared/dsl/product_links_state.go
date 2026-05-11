package itdsl

import "github.com/stretchr/testify/require"

// ProductUser returns the product user context for an alias.
func (s *State) ProductUser(alias string) *ProductUser {
	s.t.Helper()
	productUser, ok := s.productUsers[alias]
	require.True(s.t, ok, "unknown product user alias '%s'", alias)
	return productUser
}

// ProductRole returns the product role context for an alias.
func (s *State) ProductRole(alias string) *ProductRole {
	s.t.Helper()
	role, ok := s.productRoles[alias]
	require.True(s.t, ok, "unknown product role alias '%s'", alias)
	return role
}

// ProductOrganization returns the product organization context for an alias.
func (s *State) ProductOrganization(alias string) *ProductOrganization {
	s.t.Helper()
	organization, ok := s.organizations[alias]
	require.True(s.t, ok, "unknown organization alias '%s'", alias)
	return organization
}

// Membership returns the organization membership link for an alias.
func (s *State) Membership(alias string) *OrganizationMembership {
	s.t.Helper()
	membership, ok := s.memberships[alias]
	require.True(s.t, ok, "unknown membership alias '%s'", alias)
	return membership
}

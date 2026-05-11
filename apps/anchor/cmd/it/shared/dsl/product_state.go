package itdsl

import "github.com/stretchr/testify/require"

// Product returns the product context for an alias.
func (s *State) Product(alias string) *ProductContext {
	s.t.Helper()

	productCtx, ok := s.products[alias]
	require.True(s.t, ok, "unknown product alias '%s'", alias)

	return productCtx
}

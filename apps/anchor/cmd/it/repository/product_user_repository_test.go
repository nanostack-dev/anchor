package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/product/user"

	"github.com/stretchr/testify/require"
)

// createProductUserWithDetails inserts a product user fixture with caller-chosen
// name/email/status and returns the full created entity, for tests that need to
// assert on those fields (filters, sorting, full-text search).
func createProductUserWithDetails(t *testing.T, entity user.ProductUser) user.ProductUser {
	t.Helper()
	entity.GenerateID()

	created, err := repoCtx.ProductUserRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created
}

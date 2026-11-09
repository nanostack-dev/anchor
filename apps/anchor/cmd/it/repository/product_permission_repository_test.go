package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/permission"

	"github.com/stretchr/testify/require"
)

// createProductPermission inserts a product permission fixture under productID
// and returns the created domain value. Unlike the other fixtures in
// fixtures_test.go, ProductPermission has no shared helper: it is keyed by
// (product_id, name) instead of a generated ID, so the caller supplies the name.
func createProductPermission(
	t *testing.T, productID, name string, description *string,
) permission.ProductPermission {
	t.Helper()

	entity := permission.ProductPermission{
		ProductID:   productID,
		Name:        name,
		Description: description,
	}

	created, err := repoCtx.ProductPermissionRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created
}

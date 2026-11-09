package repository_test

import (
	"context"
	"testing"

	resourcepermission "anchor/internal/domain/product/resource_permission"

	"github.com/stretchr/testify/require"
)

// createResourcePermission inserts a resource permission fixture under
// productID with the given name (no description/scope modifier) and returns
// the created domain value.
func createResourcePermission(
	t *testing.T, productID, name string,
) resourcepermission.ProductResourcePermission {
	t.Helper()
	return createResourcePermissionFull(t, productID, name, nil, nil)
}

// createResourcePermissionFull inserts a resource permission fixture with an
// optional description and scope modifier.
func createResourcePermissionFull(
	t *testing.T, productID, name string, description, scopeModifier *string,
) resourcepermission.ProductResourcePermission {
	t.Helper()

	perm := resourcepermission.ProductResourcePermission{
		ProductID:      productID,
		Name:           name,
		Description:    description,
		ScopeModifier:  scopeModifier,
		ResourceTypeID: organizationTypeID(t, productID),
	}

	created, err := repoCtx.ProductResourcePermissionRepository.Create(context.Background(), perm)
	require.NoError(t, err)

	return created
}

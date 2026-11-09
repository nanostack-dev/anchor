package service_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/permission"
)

// The startup sync deletes each product permission that the server does not
// generate, and keeps the generated ones.
func TestProductPermissionStartupSyncDeletesOnlyStalePermissions(t *testing.T) {
	// Given a product with the generated permissions and one stale permission
	productID := GivenATenantAndProduct(t).Product.ID
	generated := GivenBasicAnchorPermissions(t, productID)
	stale := "stale:perm-" + uuid.NewString()
	_, err := PermissionRepository.Create(
		t.Context(), permission.ProductPermission{ProductID: productID, Name: stale},
	)
	require.NoError(t, err)

	// When the startup sync runs
	require.NoError(t, PermissionStartupSync.Start(t.Context()))

	// Then the stale permission is deleted and the generated ones stay
	found, err := PermissionRepository.FindByProductIDAndPermissionName(t.Context(), productID, stale)
	require.NoError(t, err)
	assert.False(t, found.IsPresent(), "stale permission %q must be deleted", stale)

	kept, err := PermissionRepository.FindByProductIDAndPermissionNames(t.Context(), productID, generated)
	require.NoError(t, err)
	assert.Len(t, kept, len(generated))
}

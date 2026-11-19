package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/fgasync"
	"anchor/internal/authz/model"
	"anchor/internal/domain/product/role"
	"anchor/internal/domain/product/user"
)

// TestAuthzCascadeMarks covers the deletes whose memberships cascade without
// membership events: they mark only the Organizations those memberships were
// in, not every Organization of the Product.
func TestAuthzCascadeMarks(t *testing.T) {
	openFGA(t)

	t.Run("Deleting a member of one organization of three syncs only that organization", func(t *testing.T) {
		w := givenSyncWorld(t)
		otherA, otherB := givenOrganization(t, w.productID), givenOrganization(t, w.productID)
		userID := givenMember(t, w, w.organizationID, w.managedID)
		staying := givenMember(t, w, otherA, w.managedID)
		syncProduct(t, w.productID)
		require.True(t, can(t, w.productID, model.User(userID), w.permission, w.organizationID))
		ctx, marked := fgasync.CollectMarks(t.Context())

		require.NoError(t, ProductUserService.Delete(ctx, user.DeleteProductUserInput{
			ProductID: w.productID, ProductUserID: userID,
		}))
		AuthzSyncer.Replay(t.Context(), marked())

		assert.Equal(t, []fgasync.Scope{{ProductID: w.productID, Source: assignments, Key: w.organizationID}},
			marked(), "only the organization the user belonged to is marked")
		assert.False(t, can(t, w.productID, model.User(userID), w.permission, w.organizationID))
		assert.True(t, can(t, w.productID, model.User(staying), w.permission, otherA))
		assert.False(t, can(t, w.productID, model.User(userID), w.permission, otherB))
	})

	t.Run("Deleting a product user that belongs to no organization marks no assignments", func(t *testing.T) {
		w := givenSyncWorld(t)
		userID := givenProductUser(t, w.productID)
		ctx, marked := fgasync.CollectMarks(t.Context())

		require.NoError(t, ProductUserService.Delete(ctx, user.DeleteProductUserInput{
			ProductID: w.productID, ProductUserID: userID,
		}))

		assert.Empty(t, marked())
	})

	t.Run("Deleting a managed role that no member holds marks no assignments", func(t *testing.T) {
		w := givenSyncWorld(t)
		unused := givenManagedRole(t, w.productID, "Unused", w.permission)
		syncProduct(t, w.productID)
		ctx, marked := fgasync.CollectMarks(t.Context())

		require.NoError(t, ProductRoleService.DeleteProductRole(ctx, role.DeleteProductRoleInput{
			ProductID: w.productID, ID: unused,
		}))

		for _, scope := range marked() {
			assert.NotEqual(t, assignments, scope.Source, "the role delete marked %v", scope)
		}
	})
}

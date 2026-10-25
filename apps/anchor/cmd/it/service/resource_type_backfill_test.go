package service_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/domain/product/role"
	"anchor/internal/domain/resourcetype"
	rtservice "anchor/internal/resourcetype/service"
)

// A product, role and permission written by a release without resource types
// have no organization type and no scope. The startup backfill is the only
// path that repairs them, and no route reaches it.
func TestOrganizationBackfill(t *testing.T) {
	t.Run(
		"A product from before resource types gets its organization type, and its role and permission are scoped to it",
		func(t *testing.T) {
			ctx := t.Context()
			productID := GivenATenantAndProduct(t).Product.ID
			permissionName := "legacy:read-" + uuid.NewString()
			_, err := ResourcePermissionRepo.Create(ctx, resourcepermission.ProductResourcePermission{
				ProductID: productID, Name: permissionName,
			})
			require.NoError(t, err)
			legacyRole := role.ProductRole{ProductID: productID, Name: "Legacy " + uuid.NewString()}
			legacyRole.GenerateID()
			_, err = ProductRoleRepo.Create(ctx, legacyRole)
			require.NoError(t, err)
			before, err := ResourceTypeRepo.FindTree(ctx, productID)
			require.NoError(t, err)
			require.Empty(t, before)

			require.NoError(t, rtservice.BackfillOrganizations(ctx, ProductRepository, ResourceTypeRepo, TestLogger))

			types, err := ResourceTypeRepo.FindTree(ctx, productID)
			require.NoError(t, err)
			require.Len(t, types, 1)
			root := types[0]
			assert.Equal(t, resourcetype.OrganizationSlug, root.Slug)
			storedPermission, err := ResourcePermissionRepo.FindByName(ctx, productID, permissionName)
			require.NoError(t, err)
			require.NotNil(t, storedPermission.Value().ResourceTypeID)
			assert.Equal(t, root.ID, *storedPermission.Value().ResourceTypeID)
			storedRole, err := ProductRoleRepo.FindByProductIDAndRoleID(ctx, productID, legacyRole.ID)
			require.NoError(t, err)
			require.NotNil(t, storedRole.Value().ResourceTypeID)
			assert.Equal(t, root.ID, *storedRole.Value().ResourceTypeID)
			usage, err := ResourceTypeRepo.CountUsage(ctx, productID, root.ID)
			require.NoError(t, err)
			assert.Equal(t, resourcetype.Usage{Roles: 1, Permissions: 1}, usage)
		},
	)

	t.Run("Running the backfill again changes nothing", func(t *testing.T) {
		ctx := t.Context()
		productID := GivenATenantAndProduct(t).Product.ID
		require.NoError(t, rtservice.BackfillOrganizations(ctx, ProductRepository, ResourceTypeRepo, TestLogger))
		first, err := ResourceTypeRepo.FindTree(ctx, productID)
		require.NoError(t, err)

		require.NoError(t, rtservice.BackfillOrganizations(ctx, ProductRepository, ResourceTypeRepo, TestLogger))

		second, err := ResourceTypeRepo.FindTree(ctx, productID)
		require.NoError(t, err)
		assert.Equal(t, first, second)
	})
}

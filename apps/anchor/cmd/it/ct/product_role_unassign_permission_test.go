package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/nanostack-dev/shared/toolkit"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductRole_UnassignPermission(t *testing.T) {
	ctx := context.Background()
	testCtx := createTestProductContext(t)
	testCtx.CreateDefaultProductResourcePermissions(t)
	productID := testCtx.ProductID

	perm1 := testCtx.DefaultResourcePermissions[0].Name
	perm2 := testCtx.DefaultResourcePermissions[1].Name
	perm3 := testCtx.DefaultResourcePermissions[2].Name
	t.Run(
		"UnassignPermissionFromProductRole", func(t *testing.T) {
			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, createRoleResp.StatusCode())

			roleID := createRoleResp.JSON201.Id

			assignResp, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, roleID, ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: perm2,
				},
			)
			require.NoError(t, err)
			require.Equal(t, 204, assignResp.StatusCode())

			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			require.Equal(t, 200, getResp.StatusCode())
			require.Len(t, getResp.JSON200.Permissions, 1)

			unassignResp, err := testCtx.OwnerAuthenticatedClient().UnassignPermissionFromProductRoleWithResponse(
				ctx, productID, roleID, perm2,
			)
			require.NoError(t, err)
			assert.Equal(t, 204, unassignResp.StatusCode())

			getResp2, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 200, getResp2.StatusCode())
			assert.NotNil(t, getResp2.JSON200)
			assert.Empty(t, getResp2.JSON200.Permissions)
		},
	)

	t.Run(
		"UnassignOneOfMultiplePermissions", func(t *testing.T) {
			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "MultiPermRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, createRoleResp.StatusCode())

			roleID := createRoleResp.JSON201.Id

			for _, perm := range []string{perm1, perm2, perm3} {
				assignResp, assignErr := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
					ctx, productID, roleID, ct.AssignPermissionToProductRoleJSONRequestBody{
						PermissionName: perm,
					},
				)
				require.NoError(t, assignErr)
				require.Equal(t, 204, assignResp.StatusCode())
			}

			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			require.Equal(t, 200, getResp.StatusCode())
			require.Len(t, getResp.JSON200.Permissions, 3)

			unassignResp, err := testCtx.OwnerAuthenticatedClient().UnassignPermissionFromProductRoleWithResponse(
				ctx, productID, roleID, perm2,
			)
			require.NoError(t, err)
			assert.Equal(t, 204, unassignResp.StatusCode())

			getResp2, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 200, getResp2.StatusCode())
			assert.Len(t, getResp2.JSON200.Permissions, 2)

			permissionNames := make(map[string]bool)
			for _, perm := range getResp2.JSON200.Permissions {
				permissionNames[perm.PermissionName] = true
			}
			assert.True(t, permissionNames[perm1])
			assert.False(t, permissionNames[perm2])
			assert.True(t, permissionNames[perm3])
		},
	)

	t.Run(
		"UnassignNonexistentPermissionFromProductRole", func(t *testing.T) {
			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(t, 201, createRoleResp.StatusCode())

			roleID := createRoleResp.JSON201.Id

			nonexistentPermission := itshared.Faker.Lorem().Word() + ":" + itshared.Faker.Lorem().Word()
			unassignResp, err := testCtx.OwnerAuthenticatedClient().UnassignPermissionFromProductRoleWithResponse(
				ctx, productID, roleID, nonexistentPermission,
			)
			require.NoError(t, err)
			assert.Equal(t, 400, unassignResp.StatusCode())
			assert.Contains(t, unassignResp.JSON400.Errors[0].Code, "PERMISSIONS_NOT_FOUND")
			assert.Contains(
				t, unassignResp.JSON400.Errors[0].Message, "Product permission does not exist",
			)
		},
	)

	t.Run(
		"UnassignPermissionNotAssignedToRole", func(t *testing.T) {
			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(t, 201, createRoleResp.StatusCode())

			roleID := createRoleResp.JSON201.Id

			unassignResp, err := testCtx.OwnerAuthenticatedClient().UnassignPermissionFromProductRoleWithResponse(
				ctx, productID, roleID, perm3,
			)
			require.NoError(t, err)
			assert.Equal(t, 204, unassignResp.StatusCode())
		},
	)

	t.Run(
		"UnassignPermissionFromNonexistentProductRole", func(t *testing.T) {
			nonExistentRoleID := toolkit.NewID("productroleservice")

			unassignResp, err := testCtx.OwnerAuthenticatedClient().UnassignPermissionFromProductRoleWithResponse(
				ctx, productID, nonExistentRoleID, perm3,
			)
			require.NoError(t, err)
			assert.Equal(t, 404, unassignResp.StatusCode())
		},
	)

	t.Run(
		"UnassignPermissionWithInvalidRoleID", func(t *testing.T) {
			invalidRoleID := "invalid-role-id"

			unassignResp, err := testCtx.OwnerAuthenticatedClient().UnassignPermissionFromProductRoleWithResponse(
				ctx, productID, invalidRoleID, perm2,
			)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, unassignResp.StatusCode(), 400)
		},
	)

	t.Run(
		"UnassignPermissionForNonexistentProduct", func(t *testing.T) {
			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(t, 201, createRoleResp.StatusCode())

			nonExistentProductID := toolkit.NewID("prd")

			unassignResp, err := testCtx.OwnerAuthenticatedClient().UnassignPermissionFromProductRoleWithResponse(
				ctx, nonExistentProductID, createRoleResp.JSON201.Id, perm3,
			)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, unassignResp.StatusCode(), 400)
		},
	)

	t.Run(
		"UnassignPermissionFromRoleInDifferentProduct", func(t *testing.T) {
			anotherTestCtx := createTestProductContext(t)
			anotherProductID := anotherTestCtx.ProductID

			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(t, 201, createRoleResp.StatusCode())

			roleID := createRoleResp.JSON201.Id

			assignResp, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, roleID, ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: perm2,
				},
			)
			require.NoError(t, err)
			require.Equal(t, 204, assignResp.StatusCode())

			unassignResp, err := anotherTestCtx.OwnerAuthenticatedClient().
				UnassignPermissionFromProductRoleWithResponse(
					ctx,
					anotherProductID,
					roleID,
					perm2,
				)
			require.NoError(t, err)
			assert.Equal(t, 404, unassignResp.StatusCode())

			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 200, getResp.StatusCode())
			assert.Len(t, getResp.JSON200.Permissions, 1)
		},
	)
}

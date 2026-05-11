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

func TestProductRole_AssignPermission(t *testing.T) {
	ctx := context.Background()
	testCtx := createTestProductContext(t)
	testCtx.CreateDefaultProductResourcePermissions(t)
	productID := testCtx.ProductID

	t.Run(
		"AssignPermissionToProductRole", func(t *testing.T) {
			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createRoleResp.StatusCode(),
			)

			roleID := createRoleResp.JSON201.Id

			permissionName := testCtx.DefaultResourcePermissions[0].Name

			assignResp, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, roleID, ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: permissionName,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 204, assignResp.StatusCode())

			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 200, getResp.StatusCode())
			assert.NotNil(t, getResp.JSON200)
			assert.Len(t, getResp.JSON200.Permissions, 1)
			assert.Equal(t, permissionName, getResp.JSON200.Permissions[0].PermissionName)
		},
	)

	t.Run(
		"AssignMultiplePermissionsToProductRole", func(t *testing.T) {
			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "MultiPermRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createRoleResp.StatusCode(),
			)

			roleID := createRoleResp.JSON201.Id

			assignResp1, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, roleID, ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: testCtx.DefaultResourcePermissions[0].Name,
				},
			)
			require.NoError(t, err)
			require.Equal(t, 204, assignResp1.StatusCode())

			assignResp2, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, roleID, ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: testCtx.DefaultResourcePermissions[1].Name,
				},
			)
			require.NoError(t, err)
			require.Equal(t, 204, assignResp2.StatusCode())

			assignResp3, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, roleID, ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: testCtx.DefaultResourcePermissions[3].Name,
				},
			)
			require.NoError(t, err)
			require.Equal(t, 204, assignResp3.StatusCode())

			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 200, getResp.StatusCode())
			assert.NotNil(t, getResp.JSON200)
			assert.Len(t, getResp.JSON200.Permissions, 3)

			permissionNames := make(map[string]bool)
			for _, perm := range getResp.JSON200.Permissions {
				permissionNames[perm.PermissionName] = true
			}
			assert.True(t, permissionNames[testCtx.DefaultResourcePermissions[0].Name])
			assert.True(t, permissionNames[testCtx.DefaultResourcePermissions[1].Name])
			assert.True(t, permissionNames[testCtx.DefaultResourcePermissions[3].Name])
			assert.False(t, permissionNames[testCtx.DefaultResourcePermissions[2].Name])
		},
	)

	t.Run(
		"AssignDuplicatePermissionToProductRole", func(t *testing.T) {
			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "DuplicatePermRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createRoleResp.StatusCode(),
			)

			roleID := createRoleResp.JSON201.Id

			permissionName := testCtx.DefaultResourcePermissions[0].Name

			assignResp1, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, roleID, ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: permissionName,
				},
			)
			require.NoError(t, err)
			require.Equal(t, 204, assignResp1.StatusCode())

			assignResp2, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, roleID, ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: permissionName,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 204, assignResp2.StatusCode())
		},
	)

	t.Run(
		"AssignNonexistentPermissionToProductRole", func(t *testing.T) {
			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createRoleResp.StatusCode(),
			)

			roleID := createRoleResp.JSON201.Id

			nonexistentPermission := itshared.Faker.Lorem().Word() + ":" + itshared.Faker.Lorem().Word()
			assignResp, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, roleID, ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: nonexistentPermission,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, assignResp.StatusCode())
			assert.Contains(t, assignResp.JSON400.Errors[0].Code, "PERMISSIONS_NOT_FOUND")
			assert.Contains(
				t, assignResp.JSON400.Errors[0].Message, "Product permission does not exist",
			)
		},
	)

	t.Run(
		"AssignPermissionToNonexistentProductRole", func(t *testing.T) {
			nonExistentRoleID := toolkit.NewID("productroleservice")

			permissionName := testCtx.DefaultResourcePermissions[0].Name

			assignResp, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, nonExistentRoleID,
				ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: permissionName,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 404, assignResp.StatusCode())
		},
	)

	t.Run(
		"AssignPermissionToRoleWithInvalidRoleID", func(t *testing.T) {
			invalidRoleID := "invalid-role-id"

			permissionName := testCtx.DefaultResourcePermissions[0].Name

			assignResp, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, invalidRoleID, ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: permissionName,
				},
			)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, assignResp.StatusCode(), 400)
		},
	)

	t.Run(
		"AssignPermissionWithEmptyPermissionName", func(t *testing.T) {
			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createRoleResp.StatusCode(),
			)

			roleID := createRoleResp.JSON201.Id

			assignResp, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, productID, roleID, ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: "",
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, assignResp.StatusCode())
		},
	)

	t.Run(
		"AssignPermissionForNonexistentProduct", func(t *testing.T) {
			createRoleResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + toolkit.NewID("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createRoleResp.StatusCode(),
			)

			nonExistentProductID := toolkit.NewID("prd")
			permissionName := testCtx.DefaultResourcePermissions[0].Name

			assignResp, err := testCtx.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
				ctx, nonExistentProductID, createRoleResp.JSON201.Id,
				ct.AssignPermissionToProductRoleJSONRequestBody{
					PermissionName: permissionName,
				},
			)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, assignResp.StatusCode(), 400)
		},
	)
}

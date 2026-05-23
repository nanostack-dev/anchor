package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductRole_Get(t *testing.T) {
	ctx := context.Background()
	testCtx := createTestProductContext(t)
	testCtx.CreateDefaultProductResourcePermissions(t)
	productID := testCtx.ProductID
	perm1 := testCtx.DefaultResourcePermissions[0].Name
	perm2 := testCtx.DefaultResourcePermissions[1].Name

	t.Run(
		"GetExistingProductRole", func(t *testing.T) {
			roleName := "TestRole_" + ids.MustNew("test")
			roleDesc := "Test role description"
			createResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name:        roleName,
					Description: &roleDesc,
					Permissions: []string{perm1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)
			require.NotNil(t, createResp.JSON201)

			roleID := createResp.JSON201.Id

			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 200, getResp.StatusCode())
			assert.NotNil(t, getResp.JSON200)
			assert.Equal(t, roleID, getResp.JSON200.Id)
			assert.Equal(t, roleName, getResp.JSON200.Name)
			assert.Equal(t, roleDesc, *getResp.JSON200.Description)
			assert.Equal(t, productID, getResp.JSON200.ProductId)
			assert.NotZero(t, getResp.JSON200.CreatedAt)
			assert.NotZero(t, getResp.JSON200.UpdatedAt)
			assert.NotNil(t, getResp.JSON200.Permissions)
		},
	)

	t.Run(
		"GetProductRoleWithPermissions", func(t *testing.T) {
			permissions := []string{perm1, perm2}
			createResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name:        "RoleWithPermissions_" + ids.MustNew("test"),
					Permissions: permissions,
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)

			roleID := createResp.JSON201.Id

			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 200, getResp.StatusCode())
			assert.NotNil(t, getResp.JSON200)
			assert.Len(t, getResp.JSON200.Permissions, 2)

			permissionNames := make(map[string]bool)
			for _, perm := range getResp.JSON200.Permissions {
				permissionNames[perm.PermissionName] = true
			}
			assert.True(t, permissionNames[perm1])
			assert.True(t, permissionNames[perm2])
		},
	)

	t.Run(
		"GetNonexistentProductRole", func(t *testing.T) {
			nonExistentRoleID := ids.MustNew("productroleservice")
			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, nonExistentRoleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 404, getResp.StatusCode())
		},
	)

	t.Run(
		"GetProductRoleWithInvalidID", func(t *testing.T) {
			invalidRoleID := "invalid-role-id"
			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, invalidRoleID,
			)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, getResp.StatusCode(), 400)
		},
	)

	t.Run(
		"GetProductRoleForNonexistentProduct", func(t *testing.T) {
			createResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + ids.MustNew("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)

			nonExistentProductID := ids.MustNew("prd")
			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, nonExistentProductID, createResp.JSON201.Id,
			)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, getResp.StatusCode(), 400)
		},
	)

	t.Run(
		"GetProductRoleFromDifferentProduct", func(t *testing.T) {
			anotherTestCtx := createTestProductContext(t)
			anotherProductID := anotherTestCtx.ProductID

			createResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + ids.MustNew("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)

			roleID := createResp.JSON201.Id

			getResp, err := anotherTestCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, anotherProductID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 404, getResp.StatusCode())
		},
	)
}

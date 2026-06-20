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

func TestProductResourcePermissionDeleteSuccess(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	createInput := ct.CreateProductResourcePermissionRequest{
		Name:          "file:read",
		Description:   new("Read file contents"),
		ScopeModifier: new("own"),
	}

	createResp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx,
		testProduct.ProductID,
		createInput,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode())
	assert.NotNil(t, createResp.JSON201)

	permissionName := createResp.JSON201.Name

	getResp, err := testOwnerClient(t).GetProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, permissionName,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode())

	resp, err := testOwnerClient(t).DeleteProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, permissionName,
	)
	require.NoError(t, err, "delete product resource permission request should not error")
	assert.Equal(t, http.StatusNoContent, resp.StatusCode())

	getAfterDeleteResp, err := testOwnerClient(t).GetProductResourcePermissionWithResponse(
		ctx,
		testProduct.ProductID,
		permissionName,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, getAfterDeleteResp.StatusCode())
}

func TestProductResourcePermissionDeleteNotFound(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	nonExistentPermissionName := "non:existent"

	resp, err := testOwnerClient(t).DeleteProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, nonExistentPermissionName,
	)
	require.NoError(t, err, "delete non-existent resource permission request should not error")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
}

func TestProductResourcePermissionDeleteWithNonExistentProduct(t *testing.T) {
	ctx := context.Background()

	nonExistentProductID := ids.MustNew("prod")
	permissionName := "file:read"

	resp, err := testOwnerClient(t).DeleteProductResourcePermissionWithResponse(
		ctx, nonExistentProductID, permissionName,
	)
	require.NoError(
		t, err, "delete resource permission for non-existent product request should not error",
	)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
}

func TestProductResourcePermissionDeleteAssignedToRoleCascades(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	createPermissionInput := ct.CreateProductResourcePermissionRequest{
		Name:        "file:read",
		Description: new("Read file contents"),
	}

	createPermissionResp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx,
		testProduct.ProductID,
		createPermissionInput,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createPermissionResp.StatusCode())
	assert.NotNil(t, createPermissionResp.JSON201)

	permissionName := createPermissionResp.JSON201.Name

	createRoleInput := ct.ProductRoleCreateRequest{
		Name:        "Test Role",
		Description: new("Test role with permission"),
		Permissions: []string{permissionName},
	}

	createRoleResp, err := testOwnerClient(t).CreateProductRoleWithResponse(
		ctx, testProduct.ProductID, createRoleInput,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createRoleResp.StatusCode())
	assert.NotNil(t, createRoleResp.JSON201)

	resp, err := testOwnerClient(t).DeleteProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, permissionName,
	)
	require.NoError(t, err, "delete assigned resource permission request should not error")
	assert.Equal(t, http.StatusNoContent, resp.StatusCode())

	getResp, err := testOwnerClient(t).GetProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, permissionName,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode())

	getRoleResp, err := testOwnerClient(t).GetProductRoleWithResponse(
		ctx, testProduct.ProductID, createRoleResp.JSON201.Id,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getRoleResp.StatusCode())
	assert.Empty(t, getRoleResp.JSON200.Permissions)
}

func TestProductResourcePermissionDeleteAfterUnassigningFromRole(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	createPermissionInput := ct.CreateProductResourcePermissionRequest{
		Name:        "file:read",
		Description: new("Read file contents"),
	}

	createPermissionResp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx,
		testProduct.ProductID,
		createPermissionInput,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createPermissionResp.StatusCode())
	assert.NotNil(t, createPermissionResp.JSON201)

	permissionName := createPermissionResp.JSON201.Name

	createRoleInput := ct.ProductRoleCreateRequest{
		Name:        "Test Role",
		Description: new("Test role with permission"),
		Permissions: []string{permissionName},
	}

	createRoleResp, err := testOwnerClient(t).CreateProductRoleWithResponse(
		ctx, testProduct.ProductID, createRoleInput,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createRoleResp.StatusCode())
	assert.NotNil(t, createRoleResp.JSON201)

	roleID := createRoleResp.JSON201.Id

	unassignResp, err := testOwnerClient(t).UnassignPermissionFromProductRoleWithResponse(
		ctx,
		testProduct.ProductID,
		roleID,
		permissionName,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, unassignResp.StatusCode())

	deleteResp, err := testOwnerClient(t).DeleteProductResourcePermissionWithResponse(
		ctx,
		testProduct.ProductID,
		permissionName,
	)
	require.NoError(t, err, "delete unassigned resource permission request should not error")
	assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode())

	getResp, err := testOwnerClient(t).GetProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, permissionName,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode())
}

func TestProductResourcePermissionDeleteWithInvalidPermissionName(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	testCases := []struct {
		name           string
		permissionName string
		expectedStatus int
	}{
		{
			name:           "empty permission name",
			permissionName: "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "permission name with special characters",
			permissionName: "file@read#test",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				resp, err := testOwnerClient(t).DeleteProductResourcePermissionWithResponse(
					ctx,
					testProduct.ProductID,
					tc.permissionName,
				)
				require.NoError(t, err, "request should not error")
				assert.GreaterOrEqual(
					t, resp.StatusCode(), 400,
					"should return client error for invalid permission name",
				)
			},
		)
	}
}

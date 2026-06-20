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

func TestProductResourcePermissionGetSuccess(t *testing.T) {
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

	resp, err := testOwnerClient(t).GetProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, permissionName,
	)
	require.NoError(t, err, "get product resource permission request should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.Equal(t, testProduct.ProductID, resp.JSON200.ProductId)
		assert.Equal(t, createInput.Name, resp.JSON200.Name)
		assert.Equal(t, createInput.Description, resp.JSON200.Description)
		assert.Equal(t, createInput.ScopeModifier, resp.JSON200.ScopeModifier)
		assert.NotZero(t, resp.JSON200.CreatedAt)
		assert.NotZero(t, resp.JSON200.UpdatedAt)
	}
}

func TestProductResourcePermissionGetNotFound(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	nonExistentPermissionName := "non:existent"

	resp, err := testOwnerClient(t).GetProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, nonExistentPermissionName,
	)
	require.NoError(t, err, "get non-existent resource permission request should not error")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
}

func TestProductResourcePermissionGetWithNonExistentProduct(t *testing.T) {
	ctx := context.Background()

	nonExistentProductID := ids.MustNew("prod")
	permissionName := "file:read"

	resp, err := testOwnerClient(t).GetProductResourcePermissionWithResponse(
		ctx, nonExistentProductID, permissionName,
	)
	require.NoError(
		t, err, "get resource permission for non-existent product request should not error",
	)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
}

func TestProductResourcePermissionGetWithInvalidPermissionName(t *testing.T) {
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
				resp, err := testOwnerClient(t).GetProductResourcePermissionWithResponse(
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

func TestProductResourcePermissionGetMultiplePermissions(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	permissions := []ct.CreateProductResourcePermissionRequest{
		{
			Name:          "file:read",
			Description:   new("Read file contents"),
			ScopeModifier: new("own"),
		},
		{
			Name:          "file:write",
			Description:   new("Write file contents"),
			ScopeModifier: new("team"),
		},
		{
			Name:        "file:delete",
			Description: new("Delete file contents"),
		},
	}

	var createdPermissions []ct.ProductResourcePermissionResponse
	for _, permission := range permissions {
		createResp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
			ctx,
			testProduct.ProductID,
			permission,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, createResp.StatusCode())
		assert.NotNil(t, createResp.JSON201)
		createdPermissions = append(createdPermissions, *createResp.JSON201)
	}

	for i, created := range createdPermissions {
		resp, err := testOwnerClient(t).GetProductResourcePermissionWithResponse(
			ctx, testProduct.ProductID, created.Name,
		)
		require.NoError(t, err, "get resource permission should not error")
		assert.Equal(t, http.StatusOK, resp.StatusCode())

		if assert.NotNil(t, resp.JSON200) {
			assert.Equal(t, testProduct.ProductID, resp.JSON200.ProductId)
			assert.Equal(t, permissions[i].Name, resp.JSON200.Name)
			assert.Equal(t, permissions[i].Description, resp.JSON200.Description)
			assert.Equal(t, permissions[i].ScopeModifier, resp.JSON200.ScopeModifier)
			assert.NotZero(t, resp.JSON200.CreatedAt)
			assert.NotZero(t, resp.JSON200.UpdatedAt)
		}
	}
}

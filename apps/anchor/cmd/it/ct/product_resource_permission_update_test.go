package ct_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductResourcePermissionUpdateSuccess(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	createInput := ct.CreateProductResourcePermissionRequest{
		Name:          "file:read",
		Description:   ptr.Ptr("Read file contents"),
		ScopeModifier: ptr.Ptr("own"),
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

	updateInput := ct.UpdateProductResourcePermissionRequest{
		Description: ptr.Ptr("Updated read file contents"),
	}

	resp, err := testOwnerClient(t).UpdateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, permissionName, updateInput,
	)
	require.NoError(t, err, "update product resource permission request should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.Equal(t, testProduct.ProductID, resp.JSON200.ProductId)
		assert.Equal(t, createInput.Name, resp.JSON200.Name)
		assert.Equal(t, updateInput.Description, resp.JSON200.Description)
		assert.Equal(t, createInput.ScopeModifier, resp.JSON200.ScopeModifier)
		assert.NotZero(t, resp.JSON200.CreatedAt)
		assert.NotZero(t, resp.JSON200.UpdatedAt)
		assert.True(
			t, resp.JSON200.UpdatedAt.After(resp.JSON200.CreatedAt),
			"updated_at should be after created_at",
		)
	}
}

func TestProductResourcePermissionUpdateDifferentCaseName(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	permissionName := "File:ReadCase:" + ids.MustNew("test")

	createResp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx,
		testProduct.ProductID,
		ct.CreateProductResourcePermissionRequest{
			Name:        permissionName,
			Description: ptr.Ptr("Read file contents"),
		},
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode())
	assert.NotNil(t, createResp.JSON201)

	updateInput := ct.UpdateProductResourcePermissionRequest{
		Description: ptr.Ptr("Updated read file contents"),
	}

	resp, err := testOwnerClient(t).UpdateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, strings.ToLower(permissionName), updateInput,
	)
	require.NoError(t, err, "update product resource permission request should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.Equal(t, permissionName, resp.JSON200.Name)
		assert.Equal(t, updateInput.Description, resp.JSON200.Description)
	}
}

func TestProductResourcePermissionUpdateNotFound(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	nonExistentPermissionName := "non:existent"
	updateInput := ct.UpdateProductResourcePermissionRequest{
		Description: ptr.Ptr("Updated description"),
	}

	resp, err := testOwnerClient(t).UpdateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, nonExistentPermissionName, updateInput,
	)
	require.NoError(t, err, "update non-existent resource permission request should not error")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
}

func TestProductResourcePermissionUpdateWithNonExistentProduct(t *testing.T) {
	ctx := context.Background()

	nonExistentProductID := ids.MustNew("prod")
	permissionName := "file:read"
	updateInput := ct.UpdateProductResourcePermissionRequest{
		Description: ptr.Ptr("Updated description"),
	}

	resp, err := testOwnerClient(t).UpdateProductResourcePermissionWithResponse(
		ctx, nonExistentProductID, permissionName, updateInput,
	)
	require.NoError(
		t, err, "update resource permission for non-existent product request should not error",
	)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
}

func TestProductResourcePermissionUpdateValidationErrors(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	createInput := ct.CreateProductResourcePermissionRequest{
		Name:        "file:read",
		Description: ptr.Ptr("Read file contents"),
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

	testCases := []struct {
		name        string
		input       ct.UpdateProductResourcePermissionRequest
		expectedErr string
	}{
		{
			name: "description too long",
			input: ct.UpdateProductResourcePermissionRequest{
				Description: ptr.Ptr(string(make([]byte, 501))),
			},
			expectedErr: "description must be at most 500 characters",
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				resp, validationErr := testOwnerClient(t).UpdateProductResourcePermissionWithResponse(
					ctx,
					testProduct.ProductID,
					permissionName,
					tc.input,
				)
				require.NoError(t, validationErr, "request should not error")
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
			},
		)
	}
}

func TestProductResourcePermissionUpdateNullDescription(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	createInput := ct.CreateProductResourcePermissionRequest{
		Name:        "file:read",
		Description: ptr.Ptr("Original description"),
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

	updateInput := ct.UpdateProductResourcePermissionRequest{
		Description: nil,
	}

	resp, err := testOwnerClient(t).UpdateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, permissionName, updateInput,
	)
	require.NoError(t, err, "update product resource permission request should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.Equal(t, testProduct.ProductID, resp.JSON200.ProductId)
		assert.Equal(t, createInput.Name, resp.JSON200.Name)
		assert.Nil(t, resp.JSON200.Description)
		assert.NotZero(t, resp.JSON200.CreatedAt)
		assert.NotZero(t, resp.JSON200.UpdatedAt)
	}
}

func TestProductResourcePermissionUpdateEmptyRequest(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	createInput := ct.CreateProductResourcePermissionRequest{
		Name:        "file:read",
		Description: ptr.Ptr("Original description"),
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
	originalCreatedAt := createResp.JSON201.CreatedAt
	originalUpdatedAt := createResp.JSON201.UpdatedAt

	updateInput := ct.UpdateProductResourcePermissionRequest{}

	resp, err := testOwnerClient(t).UpdateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, permissionName, updateInput,
	)
	require.NoError(t, err, "update product resource permission request should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.Equal(t, testProduct.ProductID, resp.JSON200.ProductId)
		assert.Equal(t, createInput.Name, resp.JSON200.Name)
		assert.Equal(t, updateInput.Description, resp.JSON200.Description)
		assert.Equal(t, originalCreatedAt, resp.JSON200.CreatedAt)

		assert.True(
			t,
			resp.JSON200.UpdatedAt.Equal(originalUpdatedAt) || resp.JSON200.UpdatedAt.After(originalUpdatedAt),
		)
	}
}

func TestProductResourcePermissionUpdateMultipleFields(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	createInput := ct.CreateProductResourcePermissionRequest{
		Name:        "file:read",
		Description: ptr.Ptr("Original description"),
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

	updateInput := ct.UpdateProductResourcePermissionRequest{
		Description: ptr.Ptr("Completely updated description with more details"),
	}

	resp, err := testOwnerClient(t).UpdateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, permissionName, updateInput,
	)
	require.NoError(t, err, "update product resource permission request should not error")
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	if assert.NotNil(t, resp.JSON200) {
		assert.Equal(t, testProduct.ProductID, resp.JSON200.ProductId)
		assert.Equal(t, createInput.Name, resp.JSON200.Name)
		assert.Equal(t, updateInput.Description, resp.JSON200.Description)
		assert.NotZero(t, resp.JSON200.CreatedAt)
		assert.NotZero(t, resp.JSON200.UpdatedAt)
		assert.True(t, resp.JSON200.UpdatedAt.After(resp.JSON200.CreatedAt))
	}
}

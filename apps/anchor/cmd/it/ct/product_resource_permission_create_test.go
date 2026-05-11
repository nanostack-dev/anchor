package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nanostack-dev/shared/toolkit"
)

func TestProductResourcePermissionCreateSuccess(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	input := ct.CreateProductResourcePermissionRequest{
		Name:          "file:read",
		Description:   toolkit.Ptr("Read file contents"),
		ScopeModifier: toolkit.Ptr("own"),
	}

	resp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, input,
	)
	require.NoError(t, err, "create product resource permission request should not error")
	assert.Equal(t, http.StatusCreated, resp.StatusCode())

	if assert.NotNil(t, resp.JSON201) {
		assert.Equal(t, testProduct.ProductID, resp.JSON201.ProductId)
		assert.Equal(t, input.Name, resp.JSON201.Name)
		assert.Equal(t, input.Description, resp.JSON201.Description)
		assert.Equal(t, input.ScopeModifier, resp.JSON201.ScopeModifier)
		assert.NotZero(t, resp.JSON201.CreatedAt)
		assert.NotZero(t, resp.JSON201.UpdatedAt)
	}
}

func TestProductResourcePermissionCreateValidationErrors(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	testCases := []struct {
		name        string
		input       ct.CreateProductResourcePermissionRequest
		expectedErr string
	}{
		{
			name: "empty name",
			input: ct.CreateProductResourcePermissionRequest{
				Name:        "",
				Description: toolkit.Ptr("Test description"),
			},
			expectedErr: "name is required",
		},
		{
			name: "name too short",
			input: ct.CreateProductResourcePermissionRequest{
				Name:        "a",
				Description: toolkit.Ptr("Test description"),
			},
			expectedErr: "name must be at least 2 characters",
		},
		{
			name: "name too long",
			input: ct.CreateProductResourcePermissionRequest{
				Name:        string(make([]byte, 201)),
				Description: toolkit.Ptr("Test description"),
			},
			expectedErr: "name must be at most 200 characters",
		},
		{
			name: "description too long",
			input: ct.CreateProductResourcePermissionRequest{
				Name:        "valid:name",
				Description: toolkit.Ptr(string(make([]byte, 501))),
			},
			expectedErr: "description must be at most 500 characters",
		},
		{
			name: "scope modifier too long",
			input: ct.CreateProductResourcePermissionRequest{
				Name:          "valid:name",
				Description:   toolkit.Ptr("Test description"),
				ScopeModifier: toolkit.Ptr(string(make([]byte, 101))),
			},
			expectedErr: "scope_modifier must be at most 100 characters",
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				resp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
					ctx,
					testProduct.ProductID,
					tc.input,
				)
				require.NoError(t, err, "request should not error")
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
			},
		)
	}
}

func TestProductResourcePermissionCreateDuplicate(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	input := ct.CreateProductResourcePermissionRequest{
		Name:        "file:duplicate",
		Description: toolkit.Ptr("First permission"),
	}

	resp1, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, input,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp1.StatusCode())

	resp2, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, input,
	)
	require.NoError(t, err, "request should not error")
	itshared.AssertAnchorBadRequestError(
		t, resp2, "RESOURCE_PERMISSION_ALREADY_EXISTS",
		"Resource permission with name 'file:duplicate' already exists", map[string]interface{}{
			"name": input.Name,
		},
	)
}

func TestProductResourcePermissionCreateNonExistentProduct(t *testing.T) {
	ctx := context.Background()

	nonExistentProductID := toolkit.NewID("prod")

	input := ct.CreateProductResourcePermissionRequest{
		Name:        "file:read",
		Description: toolkit.Ptr("Test permission"),
	}

	resp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx, nonExistentProductID, input,
	)
	require.NoError(t, err, "request should not error")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
}

func TestProductResourcePermissionCreateWithMinimalData(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	input := ct.CreateProductResourcePermissionRequest{
		Name: "minimal:permission",
	}

	resp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, input,
	)
	require.NoError(t, err, "create product resource permission request should not error")
	assert.Equal(t, http.StatusCreated, resp.StatusCode())

	if assert.NotNil(t, resp.JSON201) {
		assert.Equal(t, testProduct.ProductID, resp.JSON201.ProductId)
		assert.Equal(t, input.Name, resp.JSON201.Name)
		assert.Nil(t, resp.JSON201.Description)
		assert.Nil(t, resp.JSON201.ScopeModifier)
		assert.NotZero(t, resp.JSON201.CreatedAt)
		assert.NotZero(t, resp.JSON201.UpdatedAt)
	}
}

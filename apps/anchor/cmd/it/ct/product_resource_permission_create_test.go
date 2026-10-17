package ct_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestProductResourcePermissionCreateSuccess(t *testing.T) {
	// Creating a product resource permission succeeds.
	t.Parallel()
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	input := ct.CreateProductResourcePermissionRequest{
		Name:          "file:read",
		Description:   new("Read file contents"),
		ScopeModifier: new("own"),
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

func TestProductResourcePermissionCreateEmitsWebhook(t *testing.T) {
	// Product resource permission create emits webhook.
	t.Parallel()
	ctx := context.Background()
	productContext := createTestProductContext(t)
	sink := productContext.CaptureEvents()
	resp, err := productContext.OwnerAuthenticatedClient().CreateProductResourcePermissionWithResponse(
		ctx, productContext.ProductID, ct.CreateProductResourcePermissionRequest{
			Name: "events:read",
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode())
	sink.WaitFor("product.resource_permission.created", map[string]string{
		"permission_name": resp.JSON201.Name,
	})
}

func TestProductResourcePermissionCreateValidationErrors(t *testing.T) {
	// Checks product resource permission creation across the scenarios below.
	t.Parallel()
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	testCases := []struct {
		name        string
		input       ct.CreateProductResourcePermissionRequest
		expectedErr string
	}{
		// The permission operation rejects empty name.
		{
			name: "empty name",
			input: ct.CreateProductResourcePermissionRequest{
				Name:        "",
				Description: new("Test description"),
			},
			expectedErr: "name is required",
		},
		// The permission operation rejects name too short.
		{
			name: "name too short",
			input: ct.CreateProductResourcePermissionRequest{
				Name:        "a",
				Description: new("Test description"),
			},
			expectedErr: "name must be at least 2 characters",
		},
		// The permission operation rejects name too long.
		{
			name: "name too long",
			input: ct.CreateProductResourcePermissionRequest{
				Name:        string(make([]byte, 201)),
				Description: new("Test description"),
			},
			expectedErr: "name must be at most 200 characters",
		},
		// The permission operation rejects description too long.
		{
			name: "description too long",
			input: ct.CreateProductResourcePermissionRequest{
				Name:        "valid:name",
				Description: new(string(make([]byte, 501))),
			},
			expectedErr: "description must be at most 500 characters",
		},
		// The permission operation rejects scope modifier too long.
		{
			name: "scope modifier too long",
			input: ct.CreateProductResourcePermissionRequest{
				Name:          "valid:name",
				Description:   new("Test description"),
				ScopeModifier: new(string(make([]byte, 101))),
			},
			expectedErr: "scope_modifier must be at most 100 characters",
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				// Permission creation refuses this invalid field value.
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
	// Product resource permission creation rejects a duplicate name.
	t.Parallel()
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	input := ct.CreateProductResourcePermissionRequest{
		Name:        "file:duplicate",
		Description: new("First permission"),
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
	itshared.AssertAnchorConflictError(
		t, resp2, "RESOURCE_PERMISSION_ALREADY_EXISTS",
		"A resource permission with the name 'file:duplicate' already exists.", map[string]any{
			"name": input.Name,
		},
	)
}

func TestProductResourcePermissionCreateDuplicateDifferentCase(t *testing.T) {
	// Product resource permission creation rejects a duplicate name regardless of letter case.
	t.Parallel()
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	permissionName := "File:DuplicateCase:" + ids.MustNew("test")

	input := ct.CreateProductResourcePermissionRequest{
		Name:        permissionName,
		Description: new("First permission"),
	}

	resp1, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, input,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp1.StatusCode())

	duplicateName := strings.ToLower(permissionName)
	resp2, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, ct.CreateProductResourcePermissionRequest{
			Name:        duplicateName,
			Description: new("Second permission"),
		},
	)
	require.NoError(t, err, "request should not error")
	itshared.AssertAnchorConflictError(
		t, resp2, "RESOURCE_PERMISSION_ALREADY_EXISTS",
		"A resource permission with the name '"+duplicateName+"' already exists.", map[string]any{
			"name": duplicateName,
		},
	)
}

func TestProductResourcePermissionCreateNonExistentProduct(t *testing.T) {
	// Creating a product resource permission nonexistent product returns not found.
	t.Parallel()
	ctx := context.Background()

	nonExistentProductID := ids.MustNew("prod")

	input := ct.CreateProductResourcePermissionRequest{
		Name:        "file:read",
		Description: new("Test permission"),
	}

	resp, err := testOwnerClient(t).CreateProductResourcePermissionWithResponse(
		ctx, nonExistentProductID, input,
	)
	require.NoError(t, err, "request should not error")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode())
}

func TestProductResourcePermissionCreateWithMinimalData(t *testing.T) {
	// Creating a product resource permission with minimal data succeeds.
	t.Parallel()
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

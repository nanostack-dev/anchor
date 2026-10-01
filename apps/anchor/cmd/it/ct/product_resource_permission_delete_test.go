package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductResourcePermissionDeleteSuccess(t *testing.T) {
	t.Parallel()
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

func TestProductResourcePermissionDeleteEmitsWebhook(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	productContext := createTestProductContext(t)
	sink := productContext.CaptureEvents()
	createResp, err := productContext.OwnerAuthenticatedClient().CreateProductResourcePermissionWithResponse(
		ctx, productContext.ProductID, ct.CreateProductResourcePermissionRequest{
			Name: "events:delete",
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())
	permissionName := createResp.JSON201.Name
	sink.WaitFor("product.resource_permission.created", map[string]string{
		"permission_name": permissionName,
	})

	deleteResp, err := productContext.OwnerAuthenticatedClient().DeleteProductResourcePermissionWithResponse(
		ctx, productContext.ProductID, permissionName,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, deleteResp.StatusCode())
	sink.WaitFor("product.resource_permission.deleted", map[string]string{
		"permission_name": permissionName,
	})
}

func TestProductResourcePermissionDeleteNotFound(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	sink := testProduct.CaptureEvents()

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

	sink.WaitFor("product.resource_permission.deleted", map[string]string{"permission_name": permissionName})
	sink.WaitFor("product.role.updated", map[string]string{"role_id": createRoleResp.JSON201.Id})
}

func TestProductResourcePermissionDeleteAfterUnassigningFromRole(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// Not parallel: it holds an uncommitted assignment and waits for the delete to queue behind it.
func TestProductResourcePermissionDeleteTellsRoleAssignedConcurrently(t *testing.T) {
	ctx := context.Background()
	testProduct := createTestProductContext(t)
	sink := testProduct.CaptureEvents()
	owner := testOwnerClient(t)

	created, err := owner.CreateProductResourcePermissionWithResponse(
		ctx, testProduct.ProductID, ct.CreateProductResourcePermissionRequest{Name: "doc:share"},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, created.StatusCode())
	permissionName := created.JSON201.Name

	role, err := owner.CreateProductRoleWithResponse(
		ctx, testProduct.ProductID, ct.ProductRoleCreateRequest{Name: "Reviewer"},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, role.StatusCode())
	roleID := role.JSON201.Id

	assignInFlight, err := testDB.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = assignInFlight.Rollback() })
	_, err = assignInFlight.ExecContext(
		t.Context(),
		`INSERT INTO product_role_resource_permissions (id, product_id, product_role_id, permission_name)
		 VALUES ($1, $2, $3, $4)`,
		ids.MustNew("prrp"), testProduct.ProductID, roleID, permissionName,
	)
	require.NoError(t, err)
	var assignPID int
	require.NoError(t, assignInFlight.QueryRowContext(t.Context(), `SELECT pg_backend_pid()`).Scan(&assignPID))

	deleted := make(chan functional.Result[*ct.DeleteProductResourcePermissionResponse], 1)
	go func() {
		deleted <- functional.New(owner.DeleteProductResourcePermissionWithResponse(
			t.Context(), testProduct.ProductID, permissionName,
		))
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		scanErr := testDB.QueryRowContext(t.Context(),
			`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)))`, assignPID,
		).Scan(&blocked)
		return scanErr == nil && blocked
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, assignInFlight.Commit())

	resp, err := (<-deleted).Value()
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
	sink.WaitFor("product.role.updated", map[string]string{"role_id": roleID})
}

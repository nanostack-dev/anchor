package ct_test

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

const concurrentPermissionWrites = 8

func TestProductRole_ConcurrentAssignsKeepEveryPermission(t *testing.T) {
	t.Parallel()
	productContext := createTestProductContext(t)
	permissionNames := createRacePermissions(t, productContext)
	roleID := createEmptyRole(t, productContext)

	statuses, err := runConcurrently(permissionNames, func(permissionName string) (int, error) {
		resp, err := productContext.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
			t.Context(), productContext.ProductID, roleID,
			ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: permissionName},
		)
		if err != nil {
			return 0, err
		}
		return resp.StatusCode(), nil
	})

	require.NoError(t, err)
	assert.Equal(t, repeatStatus(http.StatusNoContent), statuses)
	assert.ElementsMatch(t, permissionNames, rolePermissionNames(t, productContext, roleID))
	assert.Equal(t, concurrentPermissionWrites, countQueuedRoleUpdatedEvents(t, productContext.ProductID, roleID))
}

func TestProductRole_ConcurrentUnassignsRemoveEveryPermission(t *testing.T) {
	t.Parallel()
	productContext := createTestProductContext(t)
	permissionNames := createRacePermissions(t, productContext)
	roleID := createEmptyRole(t, productContext)
	for _, permissionName := range permissionNames {
		resp, assignErr := productContext.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
			t.Context(), productContext.ProductID, roleID,
			ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: permissionName},
		)
		require.NoError(t, assignErr)
		require.Equal(t, http.StatusNoContent, resp.StatusCode())
	}

	statuses, err := runConcurrently(permissionNames, func(permissionName string) (int, error) {
		resp, err := productContext.OwnerAuthenticatedClient().UnassignPermissionFromProductRoleWithResponse(
			t.Context(), productContext.ProductID, roleID, permissionName,
		)
		if err != nil {
			return 0, err
		}
		return resp.StatusCode(), nil
	})

	require.NoError(t, err)
	assert.Equal(t, repeatStatus(http.StatusNoContent), statuses)
	assert.Empty(t, rolePermissionNames(t, productContext, roleID))
	assert.Equal(t, 2*concurrentPermissionWrites, countQueuedRoleUpdatedEvents(t, productContext.ProductID, roleID))
}

func TestProductRole_ConcurrentDuplicateAssignsEmitOnce(t *testing.T) {
	t.Parallel()
	productContext := createTestProductContext(t)
	permissionName := createRacePermissions(t, productContext)[0]
	roleID := createEmptyRole(t, productContext)
	sameName := make([]string, concurrentPermissionWrites)
	for i := range sameName {
		sameName[i] = permissionName
	}

	statuses, err := runConcurrently(sameName, func(name string) (int, error) {
		resp, err := productContext.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
			t.Context(), productContext.ProductID, roleID,
			ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: name},
		)
		if err != nil {
			return 0, err
		}
		return resp.StatusCode(), nil
	})

	require.NoError(t, err)
	assert.Equal(t, repeatStatus(http.StatusNoContent), statuses)
	assert.Equal(t, []string{permissionName}, rolePermissionNames(t, productContext, roleID))
	assert.Equal(t, 1, countQueuedRoleUpdatedEvents(t, productContext.ProductID, roleID))
}

func createRacePermissions(t *testing.T, productContext *itdsl.ProductContext) []string {
	t.Helper()
	names := make([]string, concurrentPermissionWrites)
	for i := range names {
		names[i] = fmt.Sprintf("race:perm%d", i)
	}
	created := productContext.CreateProductResourcePermissions(t, names...)
	return functional.Slice(created).Map(func(permission ct.ProductResourcePermissionResponse) string {
		return permission.Name
	})
}

func createEmptyRole(t *testing.T, productContext *itdsl.ProductContext) string {
	t.Helper()
	resp, err := productContext.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
		t.Context(), productContext.ProductID,
		ct.CreateProductRoleJSONRequestBody{Name: "RaceRole_" + ids.MustNew("test")},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode())
	return resp.JSON201.Id
}

func runConcurrently(inputs []string, call func(string) (int, error)) ([]int, error) {
	statuses := make([]int, len(inputs))
	errs := make([]error, len(inputs))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, input := range inputs {
		wg.Go(func() {
			<-start
			statuses[i], errs[i] = call(input)
		})
	}
	close(start)
	wg.Wait()
	return statuses, errors.Join(errs...)
}

func repeatStatus(status int) []int {
	statuses := make([]int, concurrentPermissionWrites)
	for i := range statuses {
		statuses[i] = status
	}
	return statuses
}

func rolePermissionNames(t *testing.T, productContext *itdsl.ProductContext, roleID string) []string {
	t.Helper()
	resp, err := productContext.OwnerAuthenticatedClient().GetProductRoleWithResponse(
		t.Context(), productContext.ProductID, roleID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
	return functional.Slice(resp.JSON200.Permissions).Map(func(permission ct.ProductRolePermissionResponse) string {
		return permission.PermissionName
	})
}

func countQueuedRoleUpdatedEvents(t *testing.T, productID, roleID string) int {
	t.Helper()
	var count int
	err := testDB.QueryRowContext(t.Context(), `
		SELECT COUNT(*) FROM pgqueue_jobs
		WHERE queue_name = 'product-events'
		  AND convert_from(payload, 'UTF8')::jsonb ->> 'product_id' = $1
		  AND convert_from(payload, 'UTF8')::jsonb ->> 'type' = 'product.role.updated'
		  AND convert_from(payload, 'UTF8')::jsonb -> 'body' -> 'data' ->> 'role_id' = $2`,
		productID, roleID,
	).Scan(&count)
	require.NoError(t, err)
	return count
}

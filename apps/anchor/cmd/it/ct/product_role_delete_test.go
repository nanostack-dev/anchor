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

func TestProductRole_Delete(t *testing.T) {
	// Checks product role deletion across the scenarios below.
	t.Parallel()
	ctx := context.Background()
	testCtx := createTestProductContext(t)
	testCtx.CreateDefaultProductResourcePermissions(t)
	productID := testCtx.ProductID

	t.Run(
		"DeleteExistingProductRole", func(t *testing.T) {
			// Deleting existing product role succeeds.
			roleName := "RoleToDelete_" + ids.MustNew("test")
			createResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: roleName,
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)

			roleID := createResp.JSON201.Id

			deleteResp, err := testCtx.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 204, deleteResp.StatusCode())

			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 404, getResp.StatusCode())
		},
	)

	t.Run(
		"DeleteProductRoleWithPermissions", func(t *testing.T) {
			// Deleting a product role with permissions succeeds.
			permissions := []string{
				testCtx.DefaultResourcePermissions[0].Name,
				testCtx.DefaultResourcePermissions[1].Name,
			}
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

			deleteResp, err := testCtx.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 204, deleteResp.StatusCode())

			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 404, getResp.StatusCode())
		},
	)

	t.Run(
		"DeleteNonexistentProductRole", func(t *testing.T) {
			// Deleting nonexistent product role returns not found.
			nonExistentRoleID := ids.MustNew("productroleservice")
			deleteResp, err := testCtx.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
				ctx, productID, nonExistentRoleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 404, deleteResp.StatusCode())
		},
	)

	t.Run(
		"DeleteProductRoleWithInvalidID", func(t *testing.T) {
			// Deleting a product role with invalid ID returns a bad request.
			invalidRoleID := "invalid-role-id"
			deleteResp, err := testCtx.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
				ctx, productID, invalidRoleID,
			)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, deleteResp.StatusCode(), 400)
		},
	)

	t.Run(
		"DeleteProductRoleForNonexistentProduct", func(t *testing.T) {
			// Deleting a product role for nonexistent product returns a bad request.
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
			deleteResp, err := testCtx.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
				ctx, nonExistentProductID, createResp.JSON201.Id,
			)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, deleteResp.StatusCode(), 400)
		},
	)

	t.Run(
		"DeleteProductRoleFromDifferentProduct", func(t *testing.T) {
			// Deleting a role through another product returns not found and preserves the original role.
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

			deleteResp, err := anotherTestCtx.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
				ctx, anotherProductID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 404, deleteResp.StatusCode())

			getResp, err := testCtx.OwnerAuthenticatedClient().GetProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 200, getResp.StatusCode())
		},
	)

	t.Run(
		"DeleteProductRoleMultipleTimes", func(t *testing.T) {
			// Product role multiple deletion times.
			createResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "RoleToDeleteTwice_" + ids.MustNew("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)

			roleID := createResp.JSON201.Id

			deleteResp1, err := testCtx.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 204, deleteResp1.StatusCode())

			deleteResp2, err := testCtx.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
				ctx, productID, roleID,
			)
			require.NoError(t, err)
			assert.Equal(t, 404, deleteResp2.StatusCode())
		},
	)

	t.Run(
		"EmitsWebhook", func(t *testing.T) {
			// A successful change emits the corresponding signed product event.
			productContext := createTestProductContext(t)
			sink := productContext.CaptureEvents()
			createResp, err := productContext.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productContext.ProductID, ct.CreateProductRoleJSONRequestBody{
					Name: "EventsRole_" + ids.MustNew("test"),
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, createResp.StatusCode())
			roleID := createResp.JSON201.Id
			sink.WaitFor("product.role.created", map[string]string{"role_id": roleID})

			deleteResp, err := productContext.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
				ctx, productContext.ProductID, roleID,
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, deleteResp.StatusCode())
			sink.WaitFor("product.role.deleted", map[string]string{"role_id": roleID})
		},
	)
}

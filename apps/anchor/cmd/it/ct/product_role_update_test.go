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
)

func TestProductRole_Update(t *testing.T) {
	ctx := context.Background()
	testCtx := createTestProductContext(t)
	productID := testCtx.ProductID

	t.Run(
		"UpdateProductRoleName", func(t *testing.T) {
			originalName := "OriginalRole_" + ids.MustNew("test")
			createResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: originalName,
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)

			roleID := createResp.JSON201.Id

			updatedName := "UpdatedRole_" + ids.MustNew("test")
			updateResp, err := testCtx.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
				ctx, productID, roleID, ct.UpdateProductRoleJSONRequestBody{
					Name: &updatedName,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 200, updateResp.StatusCode())
			assert.NotNil(t, updateResp.JSON200)
			assert.Equal(t, updatedName, updateResp.JSON200.Name)
			assert.Equal(t, roleID, updateResp.JSON200.Id)
			assert.Equal(t, productID, updateResp.JSON200.ProductId)
		},
	)

	t.Run(
		"UpdateProductRoleDescription", func(t *testing.T) {
			originalDesc := "Original description"
			roleName := "TestRole_" + ids.MustNew("test")
			createResp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name:        roleName,
					Description: &originalDesc,
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)

			roleID := createResp.JSON201.Id

			updatedDesc := "Updated description"
			updateResp, err := testCtx.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
				ctx, productID, roleID, ct.UpdateProductRoleJSONRequestBody{
					Description: &updatedDesc,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 200, updateResp.StatusCode())
			assert.NotNil(t, updateResp.JSON200)
			assert.Equal(t, updatedDesc, *updateResp.JSON200.Description)
			assert.Equal(t, roleName, updateResp.JSON200.Name)
		},
	)

	t.Run(
		"UpdateProductRoleNameAndDescription", func(t *testing.T) {
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

			updatedName := "UpdatedRole_" + ids.MustNew("test")
			updatedDesc := "Updated description"
			updateResp, err := testCtx.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
				ctx, productID, roleID, ct.UpdateProductRoleJSONRequestBody{
					Name:        &updatedName,
					Description: &updatedDesc,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 200, updateResp.StatusCode())
			assert.NotNil(t, updateResp.JSON200)
			assert.Equal(t, updatedName, updateResp.JSON200.Name)
			assert.Equal(t, updatedDesc, *updateResp.JSON200.Description)
		},
	)

	t.Run(
		"UpdateProductRoleWithEmptyName", func(t *testing.T) {
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

			emptyName := ""
			updateResp, err := testCtx.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
				ctx, productID, roleID, ct.UpdateProductRoleJSONRequestBody{
					Name: &emptyName,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, updateResp.StatusCode())
			assert.NotNil(t, updateResp.JSON400)
			assert.Contains(t, updateResp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
		},
	)

	t.Run(
		"UpdateProductRoleWithInvalidNameLength", func(t *testing.T) {
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

			longName := strings.Repeat("a", 101)
			updateResp, err := testCtx.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
				ctx, productID, roleID, ct.UpdateProductRoleJSONRequestBody{
					Name: &longName,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, updateResp.StatusCode())
			assert.NotNil(t, updateResp.JSON400)
			assert.Contains(t, updateResp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
		},
	)

	t.Run(
		"UpdateProductRoleWithInvalidDescriptionLength", func(t *testing.T) {
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

			longDesc := strings.Repeat("a", 501)
			updateResp, err := testCtx.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
				ctx, productID, roleID, ct.UpdateProductRoleJSONRequestBody{
					Description: &longDesc,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, updateResp.StatusCode())
			if assert.NotNil(t, updateResp.JSON400) {
				assert.Contains(t, updateResp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
				assert.Contains(
					t, updateResp.JSON400.Errors[0].Message,
					"Description must be a maximum of 500 characters in length",
				)
			}
		},
	)

	t.Run(
		"UpdateProductRoleWithDuplicateName", func(t *testing.T) {
			role1Name := "Role1_" + ids.MustNew("test")
			role2Name := "Role2_" + ids.MustNew("test")

			createResp1, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: role1Name,
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp1.StatusCode(),
			)

			createResp2, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: role2Name,
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp2.StatusCode(),
			)

			updateResp, err := testCtx.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
				ctx, productID, createResp2.JSON201.Id, ct.UpdateProductRoleJSONRequestBody{
					Name: &role1Name,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, updateResp.StatusCode())
			assert.NotNil(t, updateResp.JSON400)
			assert.Contains(t, updateResp.JSON400.Errors[0].Code, "ROLE_NAME_DUPLICATE")
			assert.Contains(
				t, updateResp.JSON400.Errors[0].Message,
				"Product role with this name already exists in the product",
			)
		},
	)

	t.Run(
		"UpdateNonexistentProductRole", func(t *testing.T) {
			nonExistentRoleID := ids.MustNew("productroleservice")
			updatedName := "UpdatedName_" + ids.MustNew("test")
			updateResp, err := testCtx.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
				ctx, productID, nonExistentRoleID, ct.UpdateProductRoleJSONRequestBody{
					Name: &updatedName,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 404, updateResp.StatusCode())
		},
	)

	t.Run(
		"UpdateProductRoleWithInvalidID", func(t *testing.T) {
			invalidRoleID := "invalid-role-id"
			updatedName := "UpdatedName_" + ids.MustNew("test")
			updateResp, err := testCtx.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
				ctx, productID, invalidRoleID, ct.UpdateProductRoleJSONRequestBody{
					Name: &updatedName,
				},
			)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, updateResp.StatusCode(), 400)
		},
	)

	t.Run(
		"UpdateProductRoleForNonexistentProduct", func(t *testing.T) {
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
			updatedName := "UpdatedName_" + ids.MustNew("test")
			updateResp, err := testCtx.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
				ctx, nonExistentProductID, createResp.JSON201.Id,
				ct.UpdateProductRoleJSONRequestBody{
					Name: &updatedName,
				},
			)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, updateResp.StatusCode(), 400)
		},
	)
}

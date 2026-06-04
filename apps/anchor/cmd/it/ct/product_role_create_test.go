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

func TestProductRole_Create(t *testing.T) {
	ctx := context.Background()
	testCtx := createTestProductContext(t)
	testCtx.CreateDefaultProductResourcePermissions(t)
	productID := testCtx.ProductID

	t.Run(
		"SuccessfulCreateProductRole", func(t *testing.T) {
			roleName := "Editor_" + ids.MustNew("test")
			roleDesc := "Can edit content"
			resp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name:        roleName,
					Description: &roleDesc,
				},
			)
			require.NoError(t, err)
			assert.Equal(
				t, http.StatusCreated,
				resp.StatusCode(),
			)
			assert.NotNil(t, resp.JSON201)
			assert.Equal(t, roleName, resp.JSON201.Name)
			assert.Equal(t, roleDesc, *resp.JSON201.Description)
			assert.NotEmpty(t, resp.JSON201.Id)
			assert.NotZero(t, resp.JSON201.CreatedAt)
			assert.NotZero(t, resp.JSON201.UpdatedAt)
			assert.Equal(t, productID, resp.JSON201.ProductId)
		},
	)

	t.Run(
		"CreateProductRoleNameThatIsSubstringOfExistingIsAllowed", func(t *testing.T) {
			// Regression: role-name uniqueness must be an exact match, not a
			// substring match. Creating "<base>-admin" must not block "<base>".
			base := "Lead_" + ids.MustNew("test")
			longer := base + "-admin"

			respLonger, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{Name: longer},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, respLonger.StatusCode())

			respBase, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{Name: base},
			)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, respBase.StatusCode(),
				"a name that is a substring of an existing role must not be a duplicate")

			// And an exact duplicate is still rejected.
			respDup, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{Name: base},
			)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, respDup.StatusCode(),
				"an exact-name duplicate must still be rejected")
		},
	)

	t.Run(
		"CreateProductRoleWithEmptyName", func(t *testing.T) {
			resp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: "",
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, resp.StatusCode())
			assert.NotNil(t, resp.JSON400)
			assert.Contains(t, resp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
			assert.Contains(t, resp.JSON400.Errors[0].Message, "Name is a required field")
		},
	)

	t.Run(
		"CreateProductRoleWithInvalidNameLength", func(t *testing.T) {
			longName := strings.Repeat("a", 101)
			resp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: longName,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, resp.StatusCode())
			assert.NotNil(t, resp.JSON400)
			assert.Contains(t, resp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
			assert.Contains(
				t, resp.JSON400.Errors[0].Message,
				"Name must be a maximum of 100 characters in length",
			)
		},
	)

	t.Run(
		"CreateProductRoleWithInvalidDescriptionLength", func(t *testing.T) {
			longDesc := strings.Repeat("a", 501)
			resp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name:        "ValidRole_" + ids.MustNew("test"),
					Description: &longDesc,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, resp.StatusCode())
			assert.NotNil(t, resp.JSON400)
			assert.Contains(t, resp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
			assert.Contains(
				t, resp.JSON400.Errors[0].Message,
				"Description must be a maximum of 500 characters in length",
			)
		},
	)

	t.Run(
		"CreateProductRoleWithDuplicateName", func(t *testing.T) {
			roleName := "DuplicateRole_" + ids.MustNew("test")

			resp1, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: roleName,
				},
			)
			require.NoError(t, err)
			assert.Equal(
				t, http.StatusCreated,
				resp1.StatusCode(),
			)

			resp2, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name: roleName,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, resp2.StatusCode())
			assert.NotNil(t, resp2.JSON400)
			assert.Contains(t, resp2.JSON400.Errors[0].Code, "ROLE_NAME_DUPLICATE")
			assert.Contains(
				t, resp2.JSON400.Errors[0].Message,
				"Product role with this name already exists in the product",
			)
		},
	)

	t.Run(
		"CreateProductRoleWithDuplicateNameDifferentCase", func(t *testing.T) {
			roleName := "DuplicateRoleCase_" + ids.MustNew("test")

			resp1, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{Name: roleName},
			)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, resp1.StatusCode())

			resp2, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{Name: strings.ToLower(roleName)},
			)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp2.StatusCode())
			assert.NotNil(t, resp2.JSON400)
			assert.Contains(t, resp2.JSON400.Errors[0].Code, "ROLE_NAME_DUPLICATE")
		},
	)

	t.Run(
		"CreateProductRoleWithPermissions", func(t *testing.T) {
			perm1 := testCtx.DefaultResourcePermissions[0].Name
			perm2 := testCtx.DefaultResourcePermissions[1].Name

			permissions := []string{perm1, perm2}
			resp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name:        "RoleWithPermissions_" + ids.MustNew("test"),
					Permissions: permissions,
				},
			)
			require.NoError(t, err)
			assert.Equal(
				t, http.StatusCreated,
				resp.StatusCode(),
			)
			if assert.NotNil(t, resp.JSON201) {
				assert.NotNil(t, resp.JSON201.Permissions)
				assert.Len(t, resp.JSON201.Permissions, 2)
			}
		},
	)

	t.Run(
		"CreateProductRoleWithPermissionsDifferentCase", func(t *testing.T) {
			permissionName := testCtx.DefaultResourcePermissions[0].Name

			resp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name:        "RoleWithPermissionsCase_" + ids.MustNew("test"),
					Permissions: []string{strings.ToUpper(permissionName)},
				},
			)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, resp.StatusCode())
			if assert.NotNil(t, resp.JSON201) && assert.Len(t, resp.JSON201.Permissions, 1) {
				assert.Equal(t, permissionName, resp.JSON201.Permissions[0].PermissionName)
			}
		},
	)

	t.Run("CreateProductRoleWithAPIKeyScope", func(t *testing.T) {
		apiKeyClient, _ := testCtx.CreateAPIKeyClientWithScopes([]string{"product_role:create"})

		roleName := "APIKeyRole_" + ids.MustNew("test")
		resp, err := apiKeyClient.CreateProductRoleWithResponse(
			ctx,
			productID,
			ct.CreateProductRoleJSONRequestBody{Name: roleName},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode())
		require.NotNil(t, resp.JSON201)
		assert.Equal(t, roleName, resp.JSON201.Name)
	})

	t.Run(
		"CreateProductRoleWithNonexistentPermissions", func(t *testing.T) {
			permissions := []string{itshared.Faker.Lorem().Word() + ":" + itshared.Faker.Lorem().Word()}
			resp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, productID, ct.CreateProductRoleJSONRequestBody{
					Name:        "RoleWithBadPermissions_" + ids.MustNew("test"),
					Permissions: permissions,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, resp.StatusCode())
			if assert.NotNil(t, resp.JSON400) {
				assert.Contains(t, resp.JSON400.Errors[0].Code, "PERMISSIONS_NOT_FOUND")
				assert.Contains(
					t, resp.JSON400.Errors[0].Message, "Product permission does not exist",
				)
			}
		},
	)

	t.Run(
		"CreateProductRoleForNonexistentProduct", func(t *testing.T) {
			nonExistentProductID := ids.MustNew("prd")
			resp, err := testCtx.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				ctx, nonExistentProductID, ct.CreateProductRoleJSONRequestBody{
					Name: "TestRole_" + ids.MustNew("test"),
				},
			)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, resp.StatusCode(), 400)
		},
	)
}

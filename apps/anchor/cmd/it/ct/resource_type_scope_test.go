package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

func createScopedRole(
	t *testing.T, product *itdsl.ProductContext, resourceType string, permissions ...string,
) ct.ProductRoleResponse {
	t.Helper()
	resp, err := product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
		context.Background(), product.ProductID, ct.CreateProductRoleJSONRequestBody{
			Name: "role-" + ids.MustNew("ct"), ResourceType: resourceType, Permissions: permissions,
		})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return *resp.JSON201
}

func TestCreatePermissionWithoutResourceType(t *testing.T) {
	t.Parallel()
	t.Run("A permission that names no resource type answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp, err := product.OwnerAuthenticatedClient().CreateProductResourcePermissionWithResponse(
			t.Context(), product.ProductID, ct.CreateProductResourcePermissionJSONRequestBody{Name: "document:archive"})
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_REQUIRED")
	})
}

func TestCreatePermissionWithUnknownResourceType(t *testing.T) {
	t.Parallel()
	t.Run("A permission scoped to an undeclared type answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp, err := product.OwnerAuthenticatedClient().CreateProductResourcePermissionWithResponse(
			t.Context(), product.ProductID,
			ct.CreateProductResourcePermissionJSONRequestBody{Name: "document:archive", ResourceType: "document"})
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_NOT_FOUND")
	})
}

func TestCreatePermissionScopedToType(t *testing.T) {
	t.Parallel()
	t.Run("A permission keeps the resource type it is scoped to", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)

		created := product.CreateScopedPermissions(t, "document", "document:view")[0]

		assert.Equal(t, "document", created.ResourceType)
		read, err := product.OwnerAuthenticatedClient().GetProductResourcePermissionWithResponse(
			t.Context(), product.ProductID, "document:view")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, read.StatusCode())
		assert.Equal(t, "document", read.JSON200.ResourceType)
	})
}

func TestUpdatePermissionResourceType(t *testing.T) {
	t.Parallel()
	t.Run("Changing a permission's resource type answers 409, and naming the same one succeeds", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		product.CreateScopedPermissions(t, "document", "document:share")
		client := product.OwnerAuthenticatedClient()

		moved, err := client.UpdateProductResourcePermissionWithResponse(
			t.Context(),
			product.ProductID,
			"document:share",
			ct.UpdateProductResourcePermissionJSONRequestBody{ResourceType: new("folder")},
		)
		require.NoError(t, err)
		requireAPIError(t, moved, moved.Body, http.StatusConflict, "PERMISSION_SCOPE_IMMUTABLE")

		same, err := client.UpdateProductResourcePermissionWithResponse(
			t.Context(),
			product.ProductID,
			"document:share",
			ct.UpdateProductResourcePermissionJSONRequestBody{
				ResourceType: new("document"),
				Description:  new("Share it."),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, same.StatusCode(), string(same.Body))
		assert.Equal(t, "document", same.JSON200.ResourceType)
	})
}

func TestSearchPermissionsByResourceType(t *testing.T) {
	t.Parallel()
	t.Run("The resource_types filter keeps only permissions of those types", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		product.CreateScopedPermissions(t, "document", "document:view")
		product.CreateScopedPermissions(t, "folder", "folder:view")
		product.CreateProductResourcePermissions(t, "billing:read")

		resp, err := product.OwnerAuthenticatedClient().SearchProductResourcePermissionsWithResponse(
			t.Context(), product.ProductID, ct.SearchProductResourcePermissionsJSONRequestBody{
				Filter: &ct.ProductResourcePermissionFilter{ResourceTypes: []string{"document", "organization"}},
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())

		names := []string{}
		for _, item := range resp.JSON200.Items {
			names = append(names, item.Name)
		}
		assert.ElementsMatch(t, []string{"document:view", "billing:read"}, names)
	})
}

func TestCreateRoleWithoutResourceType(t *testing.T) {
	t.Parallel()
	t.Run("A role that names no resource type answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp, err := product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
			t.Context(), product.ProductID, ct.CreateProductRoleJSONRequestBody{Name: "reviewer-" + ids.MustNew("ct")})
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_REQUIRED")
	})
}

func TestCreateRoleBundlingPermissionsBelowItsScope(t *testing.T) {
	t.Parallel()
	t.Run("A project role bundles project, folder and document permissions", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		product.CreateScopedPermissions(t, "project", "project:manage")
		product.CreateScopedPermissions(t, "folder", "folder:edit")
		product.CreateScopedPermissions(t, "document", "document:edit")

		created := createScopedRole(t, product, "project", "project:manage", "folder:edit", "document:edit")

		assert.Equal(t, "project", created.ResourceType)
		assert.Len(t, created.Permissions, 3)
	})
}

func TestCreateRoleBundlingPermissionAboveItsScope(t *testing.T) {
	t.Parallel()
	t.Run("A project role that bundles an organization permission answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		product.CreateProductResourcePermissions(t, "members:update")

		resp, err := product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
			t.Context(), product.ProductID, ct.CreateProductRoleJSONRequestBody{
				Name: "project-admin-" + ids.MustNew(
					"ct",
				),
				ResourceType: "project",
				Permissions:  []string{"members:update"},
			})
		require.NoError(t, err)

		meta := requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "PERMISSION_OUTSIDE_ROLE_SCOPE")
		assert.Equal(t, "organization", meta["permission_resource_type"])
		assert.Equal(t, "project", meta["role_resource_type"])
	})
}

func TestAssignPermissionOutsideRoleScope(t *testing.T) {
	t.Parallel()
	t.Run("Assigning a project permission to a document role answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		product.CreateScopedPermissions(t, "project", "project:manage")
		viewer := createScopedRole(t, product, "document")

		resp, err := product.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(
			t.Context(), product.ProductID, viewer.Id, ct.AssignPermissionToProductRoleJSONRequestBody{
				PermissionName: "project:manage",
			})
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "PERMISSION_OUTSIDE_ROLE_SCOPE")
	})
}

func TestUpdateRolePermissionsOutsideScope(t *testing.T) {
	t.Parallel()
	t.Run("Replacing a folder role's permissions with a project permission answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		product.CreateScopedPermissions(t, "project", "project:manage")
		viewer := createScopedRole(t, product, "folder")

		resp, err := product.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
			t.Context(), product.ProductID, viewer.Id, ct.UpdateProductRoleJSONRequestBody{
				Name: viewer.Name, Permissions: &[]string{"project:manage"},
			})
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "PERMISSION_OUTSIDE_ROLE_SCOPE")
		read, err := product.OwnerAuthenticatedClient().
			GetProductRoleWithResponse(t.Context(), product.ProductID, viewer.Id)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, read.StatusCode())
		assert.Empty(t, read.JSON200.Permissions)
	})
}

func TestUpdateRoleResourceType(t *testing.T) {
	t.Parallel()
	t.Run("Changing a role's resource type answers 409", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		viewer := createScopedRole(t, product, "folder")

		resp, err := product.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(
			t.Context(), product.ProductID, viewer.Id, ct.UpdateProductRoleJSONRequestBody{
				Name: viewer.Name, ResourceType: new("project"),
			})
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "ROLE_SCOPE_IMMUTABLE")
	})
}

func TestSearchRolesByResourceType(t *testing.T) {
	t.Parallel()
	t.Run("The resource_types filter keeps only roles of those types", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		folderRole := createScopedRole(t, product, "folder")
		createScopedRole(t, product, "project")

		resp, err := product.OwnerAuthenticatedClient().SearchProductRolesWithResponse(
			t.Context(), product.ProductID, ct.SearchProductRolesJSONRequestBody{
				Filter: &ct.ProductRoleFilter{ResourceTypes: []string{"folder"}},
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())

		require.Len(t, resp.JSON200.Items, 1)
		assert.Equal(t, folderRole.Id, resp.JSON200.Items[0].Id)
		assert.Equal(t, "folder", resp.JSON200.Items[0].ResourceType)
	})
}

func TestSearchRolesByUndeclaredResourceType(t *testing.T) {
	t.Parallel()
	t.Run("Filtering by a type the product lacks returns no role", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		createScopedRole(t, product, "organization")

		resp, err := product.OwnerAuthenticatedClient().SearchProductRolesWithResponse(
			t.Context(), product.ProductID, ct.SearchProductRolesJSONRequestBody{
				Filter: &ct.ProductRoleFilter{ResourceTypes: []string{"dashboard"}},
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())

		assert.Empty(t, resp.JSON200.Items)
		assert.NotNil(t, resp.JSON200.Items)
	})
}

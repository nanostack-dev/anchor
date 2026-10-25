package ct_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteResourceType(t *testing.T) {
	t.Parallel()
	t.Run("Deleting an unused type removes it", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		types := product.ResourceTypes()
		project := types.CreateUnder("project", "organization")

		types.Delete(project.Id)

		resp := types.GetRaw(project.Id)
		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_TYPE_NOT_FOUND")
	})
}

func TestDeleteResourceTypeBuiltIn(t *testing.T) {
	t.Parallel()
	t.Run("Deleting the organization type answers 409", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		types := product.ResourceTypes()

		resp := types.DeleteRaw(types.BySlug("organization").Id)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "BUILT_IN_RESOURCE_TYPE")
	})
}

func TestDeleteResourceTypeWithChildType(t *testing.T) {
	t.Parallel()
	t.Run("Deleting a type another type sits under answers 409 with the count", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		tree := declareDocumentTree(t, product)

		resp := product.ResourceTypes().DeleteRaw(tree.project.Id)

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		assert.Equal(t, map[string]any{"child_types": float64(2), "roles": float64(0), "permissions": float64(0)},
			meta["blocked_by"])
	})
}

func TestDeleteResourceTypeWithPermission(t *testing.T) {
	t.Parallel()
	t.Run("Deleting a type a permission is scoped to answers 409 with the count", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		tree := declareDocumentTree(t, product)
		product.CreateScopedPermissions(t, "document", "document:view", "document:edit")

		resp := product.ResourceTypes().DeleteRaw(tree.document.Id)

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		assert.Equal(t, map[string]any{"child_types": float64(0), "roles": float64(0), "permissions": float64(2)},
			meta["blocked_by"])
	})
}

func TestDeleteResourceTypeWithRole(t *testing.T) {
	t.Parallel()
	t.Run("Deleting a type a role is scoped to answers 409 with the count", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		tree := declareDocumentTree(t, product)
		createScopedRole(t, product, "document")

		resp := product.ResourceTypes().DeleteRaw(tree.document.Id)

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		assert.Equal(t, map[string]any{"child_types": float64(0), "roles": float64(1), "permissions": float64(0)},
			meta["blocked_by"])
	})
}

func TestDeleteProductWithResourceTypes(t *testing.T) {
	t.Parallel()
	t.Run("Deleting a product removes its nested types, scoped roles and scoped permissions", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		product.CreateScopedPermissions(t, "document", "document:view")
		createScopedRole(t, product, "document")

		resp, err := product.OwnerAuthenticatedClient().DeleteProductWithResponse(t.Context(), product.ProductID)

		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
	})
}

func TestDeleteResourceTypeEmitsEvent(t *testing.T) {
	t.Parallel()
	t.Run("Deleting a type emits product.resource_type.deleted", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		sink := product.CaptureEvents()
		project := product.ResourceTypes().CreateUnder("project", "organization")

		product.ResourceTypes().Delete(project.Id)

		sink.WaitFor("product.resource_type.deleted", map[string]string{"resource_type_id": project.Id})
	})
}

package ct_test

import (
	"net/http"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateResourceType(t *testing.T) {
	t.Parallel()
	t.Run("Renaming a type and turning nesting on keeps its slug and parents", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		tree := declareDocumentTree(t, product)

		updated := product.ResourceTypes().Update(tree.project.Id, ct.UpdateProductResourceTypeJSONRequestBody{
			Name: new("Workspace project"), NestsInItself: new(true),
		})

		assert.Equal(t, "project", updated.Slug)
		assert.Equal(t, "Workspace project", updated.Name)
		assert.True(t, updated.NestsInItself)
		assert.Equal(t, []string{"organization"}, updated.ParentTypes)
	})
}

func TestUpdateResourceTypeAddsParent(t *testing.T) {
	t.Parallel()
	t.Run("Adding a parent type lets the type sit under it", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		tree := declareDocumentTree(t, product)

		updated := product.ResourceTypes().Update(tree.folder.Id, ct.UpdateProductResourceTypeJSONRequestBody{
			ParentTypes: &[]string{"project", "organization"},
		})

		assert.ElementsMatch(t, []string{"project", "organization"}, updated.ParentTypes)
	})
}

func TestUpdateResourceTypeTurnsNestingOff(t *testing.T) {
	t.Parallel()
	t.Run("Turning nests_in_itself off succeeds while no resource nests", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		tree := declareDocumentTree(t, product)

		updated := product.ResourceTypes().Update(tree.folder.Id, ct.UpdateProductResourceTypeJSONRequestBody{
			NestsInItself: new(false),
		})

		assert.False(t, updated.NestsInItself)
	})
}

func TestUpdateResourceTypeWithoutParent(t *testing.T) {
	t.Parallel()
	t.Run("Replacing the parent types with none answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		project := product.ResourceTypes().CreateUnder("project", "organization")

		resp := product.ResourceTypes().UpdateRaw(project.Id, ct.UpdateProductResourceTypeJSONRequestBody{
			ParentTypes: &[]string{},
		})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_PARENT_REQUIRED")
	})
}

func TestUpdateResourceTypeBuiltIn(t *testing.T) {
	t.Parallel()
	t.Run("Renaming the organization type answers 409", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		types := product.ResourceTypes()
		root := types.BySlug("organization")

		resp := types.UpdateRaw(root.Id, ct.UpdateProductResourceTypeJSONRequestBody{Name: new("Company")})

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "BUILT_IN_RESOURCE_TYPE")
	})
}

func TestUpdateResourceTypeCycle(t *testing.T) {
	t.Parallel()
	t.Run("Making project sit under folder, which sits under project, answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		tree := declareDocumentTree(t, product)

		resp := product.ResourceTypes().UpdateRaw(tree.project.Id, ct.UpdateProductResourceTypeJSONRequestBody{
			ParentTypes: &[]string{"folder"},
		})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_CYCLE")
	})
}

func TestUpdateResourceTypeBesideAnOutOfScopeRole(t *testing.T) {
	t.Parallel()
	t.Run("Renaming a type succeeds while a role holds a permission outside its scope, and moving one is refused",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			tree := declareDocumentTree(t, product)
			product.CreateScopedPermissions(t, "organization", "billing:read")
			role := createScopedRole(t, product, "project")
			// A release before scopes could still assign this during the rolling deploy.
			_, err := testDB.ExecContext(t.Context(), `INSERT INTO product_role_resource_permissions
				(id, product_id, product_role_id, permission_name) VALUES ($1, $2, $3, 'billing:read')`,
				ids.MustNew("prrp"), product.ProductID, role.Id)
			require.NoError(t, err)

			renamed := product.ResourceTypes().UpdateRaw(tree.document.Id, ct.UpdateProductResourceTypeJSONRequestBody{
				Name: new("Page"),
			})
			assert.Equal(t, http.StatusOK, renamed.StatusCode(), string(renamed.Body))

			moved := product.ResourceTypes().UpdateRaw(tree.folder.Id, ct.UpdateProductResourceTypeJSONRequestBody{
				ParentTypes: &[]string{"project", "organization"},
			})
			requireAPIError(t, moved, moved.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		})
}

func TestUpdateResourceTypeRemovingParentARoleNeeds(t *testing.T) {
	t.Parallel()
	t.Run(
		"Cutting document off from project answers 409 while a project role holds a document permission",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			tree := declareDocumentTree(t, product)
			product.CreateScopedPermissions(t, "document", "document:view")
			product.ResourceTypes().Update(tree.folder.Id, ct.UpdateProductResourceTypeJSONRequestBody{
				ParentTypes: &[]string{"project", "organization"},
			})
			role := createScopedRole(t, product, "project", "document:view")

			resp := product.ResourceTypes().UpdateRaw(tree.document.Id, ct.UpdateProductResourceTypeJSONRequestBody{
				ParentTypes: &[]string{"folder"},
			})
			assert.Equal(t, http.StatusOK, resp.StatusCode(), "folder still sits under project")

			resp = product.ResourceTypes().UpdateRaw(tree.folder.Id, ct.UpdateProductResourceTypeJSONRequestBody{
				ParentTypes: &[]string{"organization"},
			})

			meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
			assert.Equal(t, []any{role.Id}, meta["role_ids"])
		},
	)
}

func TestUpdateResourceTypeEmitsEvent(t *testing.T) {
	t.Parallel()
	t.Run("Renaming a type emits product.resource_type.updated", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		sink := product.CaptureEvents()
		project := product.ResourceTypes().CreateUnder("project", "organization")

		product.ResourceTypes().Update(project.Id, ct.UpdateProductResourceTypeJSONRequestBody{Name: new("Space")})

		sink.WaitFor("product.resource_type.updated", map[string]string{"resource_type_id": project.Id})
	})
}

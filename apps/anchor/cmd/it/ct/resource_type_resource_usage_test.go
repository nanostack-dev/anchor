package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"

	itdsl "anchor/cmd/it/shared/dsl"
)

// documentTreeItems is the document tree as a whole-tree body, with the
// changes the test makes applied to it.
func documentTreeItems(documentParents []string, folderNests bool) []ct.ProductResourceTypeTreeItem {
	return []ct.ProductResourceTypeTreeItem{
		{Slug: "project", Name: "project", ParentTypes: []string{"organization"}},
		{Slug: "folder", Name: "Folder", ParentTypes: []string{"project"}, NestsInItself: new(folderNests)},
		{Slug: "document", Name: "document", ParentTypes: documentParents},
	}
}

func TestResourceTypesInUseByResources(t *testing.T) {
	t.Parallel()

	t.Run("Deleting a type registered resources use answers 409 with the count", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		w.resources.Put("document", resourceID(), "Readme", itdsl.Ref("project", w.project))
		w.resources.Put("document", resourceID(), "Notes", itdsl.Ref("project", w.project))

		resp := w.product.ResourceTypes().DeleteRaw(w.tree.document.Id)

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		assert.Equal(t, map[string]any{
			"child_types": float64(0), "roles": float64(0), "permissions": float64(0), "resources": float64(2),
		}, meta["blocked_by"])
	})

	t.Run("Removing a parent type a resource sits under answers 409 with the count", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		w.resources.Put("document", resourceID(), "Readme", itdsl.Ref("project", w.project))

		resp := w.product.ResourceTypes().UpdateRaw(w.tree.document.Id,
			ct.UpdateProductResourceTypeJSONRequestBody{ParentTypes: &[]string{"folder"}})

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		assert.Equal(t, "project", meta["parent_type"])
		assert.Equal(t, map[string]any{
			"child_types": float64(0), "roles": float64(0), "permissions": float64(0), "resources": float64(1),
		}, meta["blocked_by"])
	})

	t.Run("Removing a parent type no resource sits under succeeds", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		design := resourceID()
		w.resources.Put("folder", design, "Design", itdsl.Ref("project", w.project))
		w.resources.Put("document", resourceID(), "Pricing", itdsl.Ref("folder", design))

		updated := w.product.ResourceTypes().Update(w.tree.document.Id,
			ct.UpdateProductResourceTypeJSONRequestBody{ParentTypes: &[]string{"folder"}})

		assert.Equal(t, []string{"folder"}, updated.ParentTypes)
	})

	t.Run("Turning nesting off while a folder sits in a folder answers 409 with the count", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		design := resourceID()
		w.resources.Put("folder", design, "Design", itdsl.Ref("project", w.project))
		w.resources.Put("folder", resourceID(), "Specs", itdsl.Ref("folder", design))

		resp := w.product.ResourceTypes().UpdateRaw(w.tree.folder.Id,
			ct.UpdateProductResourceTypeJSONRequestBody{NestsInItself: new(false)})

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		assert.Equal(t, "folder", meta["parent_type"])
		assert.EqualValues(t, 1, meta["blocked_by"].(map[string]any)["resources"])
	})

	t.Run("A tree that removes a parent type a resource sits under answers 409 with the count", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		w.resources.Put("document", resourceID(), "Readme", itdsl.Ref("project", w.project))

		resp := w.product.ResourceTypes().ReplaceTreeRaw(documentTreeItems([]string{"folder"}, true)...)

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		assert.Equal(t, "project", meta["parent_type"])
	})

	t.Run(
		"A tree that turns nesting off while a folder sits in a folder answers 409 with the count",
		func(t *testing.T) {
			t.Parallel()
			w := givenProject(t)
			design := resourceID()
			w.resources.Put("folder", design, "Design", itdsl.Ref("project", w.project))
			w.resources.Put("folder", resourceID(), "Specs", itdsl.Ref("folder", design))

			resp := w.product.ResourceTypes().ReplaceTreeRaw(documentTreeItems([]string{"folder", "project"}, false)...)

			meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
			assert.Equal(t, "folder", meta["parent_type"])
		},
	)

	t.Run("A tree that leaves out a type registered resources use answers 409 with the count", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		w.resources.Put("document", resourceID(), "Readme", itdsl.Ref("project", w.project))

		resp := w.product.ResourceTypes().ReplaceTreeRaw(documentTreeItems(nil, true)[:2]...)

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		assert.EqualValues(t, 1, meta["blocked_by"].(map[string]any)["resources"])
	})
}

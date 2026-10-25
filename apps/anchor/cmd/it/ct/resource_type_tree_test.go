package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func treeItem(slug string, parents ...string) ct.ProductResourceTypeTreeItem {
	return ct.ProductResourceTypeTreeItem{Slug: slug, Name: slug, ParentTypes: parents}
}

func TestReplaceResourceTypeTree(t *testing.T) {
	t.Parallel()
	t.Run("Publishing a tree creates, updates and deletes types by slug", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		types := product.ResourceTypes()
		project := types.CreateUnder("project", "organization")
		types.CreateUnder("archive", "organization")

		published := types.ReplaceTree(
			ct.ProductResourceTypeTreeItem{Slug: "project", Name: "Space", ParentTypes: []string{"organization"}},
			treeItem("document", "folder", "project"),
			treeItem("folder", "project"),
		)

		slugs := functional.Slice(published).Map(func(rt ct.ProductResourceTypeResponse) string { return rt.Slug })
		assert.Equal(t, []string{"organization", "project", "document", "folder"}, []string(slugs))
		assert.Equal(t, project.Id, published[1].Id)
		assert.Equal(t, "Space", published[1].Name)
		assert.ElementsMatch(t, []string{"folder", "project"}, published[2].ParentTypes)
		found := types.Search(ct.SearchProductResourceTypesJSONRequestBody{
			Filter: &ct.ProductResourceTypeFilter{Slugs: []string{"archive"}},
		})
		assert.Empty(t, found.Items)
	})
}

func TestReplaceResourceTypeTreeEmpty(t *testing.T) {
	t.Parallel()
	t.Run("Publishing an empty tree leaves only organization", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)

		published := product.ResourceTypes().ReplaceTree()

		require.Len(t, published, 1)
		assert.Equal(t, "organization", published[0].Slug)
	})
}

func TestReplaceResourceTypeTreeCycle(t *testing.T) {
	t.Parallel()
	t.Run("A tree whose types are each other's parents answers 400 and changes nothing", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		types := product.ResourceTypes()
		types.CreateUnder("project", "organization")

		resp := types.ReplaceTreeRaw(treeItem("project", "folder"), treeItem("folder", "project"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_CYCLE")
		assert.Equal(t, []string{"organization"}, types.BySlug("project").ParentTypes)
	})
}

func TestReplaceResourceTypeTreeListingOrganization(t *testing.T) {
	t.Parallel()
	t.Run("A tree that lists organization answers 409", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp := product.ResourceTypes().ReplaceTreeRaw(treeItem("organization", "organization"))

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "BUILT_IN_RESOURCE_TYPE")
	})
}

func TestReplaceResourceTypeTreeDroppingUsedType(t *testing.T) {
	t.Parallel()
	t.Run("Leaving out a type a permission is scoped to answers 409 and changes nothing", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		product.CreateScopedPermissions(t, "document", "document:view")

		resp := product.ResourceTypes().
			ReplaceTreeRaw(treeItem("project", "organization"), treeItem("folder", "project"))

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		product.ResourceTypes().BySlug("document")
	})
}

func TestReplaceResourceTypeTreeEmitsEvents(t *testing.T) {
	t.Parallel()
	t.Run("Publishing emits one event per type created, updated and deleted", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		sink := product.CaptureEvents()
		types := product.ResourceTypes()
		project := types.CreateUnder("project", "organization")
		archive := types.CreateUnder("archive", "organization")

		published := types.ReplaceTree(
			ct.ProductResourceTypeTreeItem{Slug: "project", Name: "Space", ParentTypes: []string{"organization"}},
			treeItem("folder", "project"),
		)

		sink.WaitFor("product.resource_type.updated", map[string]string{"resource_type_id": project.Id})
		sink.WaitFor("product.resource_type.created", map[string]string{"resource_type_id": published[2].Id})
		sink.WaitFor("product.resource_type.deleted", map[string]string{"resource_type_id": archive.Id})
	})
}

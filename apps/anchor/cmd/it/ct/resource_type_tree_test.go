package ct_test

import (
	"fmt"
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

		before := product.ResourceTypes().Search(ct.SearchProductResourceTypesJSONRequestBody{})

		resp := product.ResourceTypes().ReplaceTreeRaw(
			ct.ProductResourceTypeTreeItem{Slug: "project", Name: "Space", ParentTypes: []string{"organization"}},
			treeItem("folder", "project"),
		)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		assert.Equal(t, before, product.ResourceTypes().Search(ct.SearchProductResourceTypesJSONRequestBody{}))
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

func TestReplaceResourceTypeTreeUnchanged(t *testing.T) {
	t.Parallel()
	t.Run("Publishing the same tree again leaves every type as it was", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		types := product.ResourceTypes()
		first := types.ReplaceTree(treeItem("project", "organization"), treeItem("folder", "project"))

		second := types.ReplaceTree(treeItem("project", "organization"), treeItem("folder", "project"))

		assert.Equal(t, first, second)
	})
}

func TestReplaceResourceTypeTreeMaxDepth(t *testing.T) {
	t.Parallel()
	t.Run("A published tree of six levels answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp := product.ResourceTypes().ReplaceTreeRaw(
			treeItem("level_2", "organization"), treeItem("level_3", "level_2"), treeItem("level_4", "level_3"),
			treeItem("level_5", "level_4"), treeItem("level_6", "level_5"),
		)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_TOO_DEEP")
	})
}

func TestReplaceResourceTypeTreeMaxOwnTypes(t *testing.T) {
	t.Parallel()
	t.Run("A published tree holds fifty types and fifty-one answer 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		items := make([]ct.ProductResourceTypeTreeItem, 0, 51)
		for i := range 51 {
			items = append(items, treeItem(fmt.Sprintf("type_%d", i), "organization"))
		}

		within := product.ResourceTypes().ReplaceTree(items[:50]...)
		beyond := product.ResourceTypes().ReplaceTreeRaw(items...)

		assert.Len(t, within, 51)
		requireAPIError(t, beyond, beyond.Body, http.StatusBadRequest, "VALIDATION_ERROR")
	})
}

func TestReplaceResourceTypeTreeLeavingRolesOutOfScope(t *testing.T) {
	t.Parallel()
	t.Run("A tree that moves document out from under a project role's permission answers 409", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		product.CreateScopedPermissions(t, "document", "document:view")
		role := createScopedRole(t, product, "project", "document:view")

		resp := product.ResourceTypes().ReplaceTreeRaw(
			treeItem("project", "organization"),
			ct.ProductResourceTypeTreeItem{
				Slug: "folder", Name: "Folder", ParentTypes: []string{"project"}, NestsInItself: new(true),
			},
			treeItem("document", "organization"),
		)

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		assert.Equal(t, []any{role.Id}, meta["role_ids"])
	})
}

package ct_test

import (
	"fmt"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateProductSeedsOrganizationType(t *testing.T) {
	t.Parallel()
	t.Run("A new product has the built-in organization type and nothing else", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		found := product.ResourceTypes().Search(ct.SearchProductResourceTypesJSONRequestBody{})

		require.Len(t, found.Items, 1)
		root := found.Items[0]
		assert.Equal(t, "organization", root.Slug)
		assert.Equal(t, "Organization", root.Name)
		assert.True(t, root.BuiltIn)
		assert.Empty(t, root.ParentTypes)
		assert.NotNil(t, root.ParentTypes)
	})
}

func TestCreateResourceType(t *testing.T) {
	t.Parallel()
	t.Run("Creating a type under organization returns it with its parent types", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		created := product.ResourceTypes().Create(ct.CreateProductResourceTypeJSONRequestBody{
			Slug:        "project",
			Name:        "Project",
			Description: new("Groups folders."),
			ParentTypes: []string{"organization"},
		})

		assert.Regexp(t, `^rtype_`, created.Id)
		assert.Equal(t, product.ProductID, created.ProductId)
		assert.Equal(t, "project", created.Slug)
		assert.Equal(t, "Project", created.Name)
		assert.Equal(t, "Groups folders.", *created.Description)
		assert.Equal(t, []string{"organization"}, created.ParentTypes)
		assert.False(t, created.NestsInItself)
		assert.False(t, created.BuiltIn)
		assert.Equal(t, created, product.ResourceTypes().Get(created.Id))
	})
}

func TestCreateResourceTypeWithSeveralParentsAndSelfNesting(t *testing.T) {
	t.Parallel()
	t.Run("A document may sit in a folder or at the project root, and a folder in a folder", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		tree := declareDocumentTree(t, product)

		assert.True(t, tree.folder.NestsInItself)
		assert.Equal(t, []string{"project"}, tree.folder.ParentTypes)
		assert.ElementsMatch(t, []string{"folder", "project"}, tree.document.ParentTypes)
	})
}

func TestCreateResourceTypeDuplicateSlug(t *testing.T) {
	t.Parallel()
	t.Run("A slug the product already has answers 409", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		types := product.ResourceTypes()
		types.CreateUnder("project", "organization")

		resp := types.CreateRaw(ct.CreateProductResourceTypeJSONRequestBody{
			Slug: "project", Name: "Another project", ParentTypes: []string{"organization"},
		})

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_SLUG_DUPLICATE")
	})
}

func TestCreateResourceTypeOrganizationSlug(t *testing.T) {
	t.Parallel()
	t.Run("Declaring a second organization type answers 409", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp := product.ResourceTypes().CreateRaw(ct.CreateProductResourceTypeJSONRequestBody{
			Slug: "organization", Name: "Organization", ParentTypes: []string{"organization"},
		})

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_SLUG_DUPLICATE")
	})
}

func TestCreateResourceTypeUnknownParent(t *testing.T) {
	t.Parallel()
	t.Run("A parent type the product has not declared answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp := product.ResourceTypes().CreateRaw(ct.CreateProductResourceTypeJSONRequestBody{
			Slug: "folder", Name: "Folder", ParentTypes: []string{"project"},
		})

		meta := requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_NOT_FOUND")
		assert.Equal(t, []any{"project"}, meta["resource_types"])
	})
}

func TestCreateResourceTypeInvalidSlug(t *testing.T) {
	t.Parallel()
	t.Run("A slug that is not a lowercase identifier answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp := product.ResourceTypes().CreateRaw(ct.CreateProductResourceTypeJSONRequestBody{
			Slug: "Folder-Type", Name: "Folder", ParentTypes: []string{"organization"},
		})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_SLUG_INVALID")
	})
}

func TestCreateResourceTypeListingItself(t *testing.T) {
	t.Parallel()
	t.Run("A type that lists itself as a parent answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		product.ResourceTypes().CreateUnder("project", "organization")

		resp := product.ResourceTypes().CreateRaw(ct.CreateProductResourceTypeJSONRequestBody{
			Slug: "folder", Name: "Folder", ParentTypes: []string{"project", "folder"},
		})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_SELF_PARENT")
	})
}

func TestCreateResourceTypeWithoutParent(t *testing.T) {
	t.Parallel()
	t.Run("A type with no parent type answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp := product.ResourceTypes().CreateRaw(ct.CreateProductResourceTypeJSONRequestBody{
			Slug: "project", Name: "Project", ParentTypes: []string{},
		})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "VALIDATION_ERROR")
	})
}

func TestCreateResourceTypeMaxDepth(t *testing.T) {
	t.Parallel()
	t.Run("A tree reaches five levels and a sixth answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		types := product.ResourceTypes()
		parent := "organization"
		for level := 2; level <= 5; level++ {
			parent = types.CreateUnder(fmt.Sprintf("level_%d", level), parent).Slug
		}

		resp := types.CreateRaw(ct.CreateProductResourceTypeJSONRequestBody{
			Slug: "level_6", Name: "Level 6", ParentTypes: []string{parent},
		})

		meta := requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_TOO_DEEP")
		assert.InDelta(t, 6, meta["depth"], 0)
		assert.InDelta(t, 5, meta["max_depth"], 0)
	})
}

func TestCreateResourceTypeMaxOwnTypes(t *testing.T) {
	t.Parallel()
	t.Run("A product declares fifty types besides organization and a fifty-first answers 409", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		types := product.ResourceTypes()
		for i := range 50 {
			types.CreateUnder(fmt.Sprintf("type_%d", i), "organization")
		}

		resp := types.CreateRaw(ct.CreateProductResourceTypeJSONRequestBody{
			Slug: "type_50", Name: "One too many", ParentTypes: []string{"organization"},
		})

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_LIMIT_REACHED")
	})
}

func TestCreateResourceTypeEmitsEvent(t *testing.T) {
	t.Parallel()
	t.Run("Creating a type emits product.resource_type.created", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		sink := product.CaptureEvents()

		created := product.ResourceTypes().CreateUnder("project", "organization")

		sink.WaitFor("product.resource_type.created", map[string]string{
			"resource_type_id": created.Id, "resource_type": "project",
		})
	})
}

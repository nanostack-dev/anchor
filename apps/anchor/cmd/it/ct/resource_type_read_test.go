package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
)

func TestGetResourceTypeNotFound(t *testing.T) {
	t.Parallel()
	t.Run("An unknown resource type ID answers 404", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp := product.ResourceTypes().GetRaw(ids.MustNew("rtype"))

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_TYPE_NOT_FOUND")
	})
}

func TestGetResourceTypeOfAnotherProduct(t *testing.T) {
	t.Parallel()
	t.Run("A type of another product answers 404", func(t *testing.T) {
		t.Parallel()
		owner := createTestProductContext(t)
		other := createTestProductContext(t)
		project := owner.ResourceTypes().CreateUnder("project", "organization")

		resp := other.ResourceTypes().GetRaw(project.Id)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_TYPE_NOT_FOUND")
	})
}

func TestSearchResourceTypesBySlug(t *testing.T) {
	t.Parallel()
	t.Run("The slug filter keeps only the named types", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		tree := declareDocumentTree(t, product)

		found := product.ResourceTypes().Search(ct.SearchProductResourceTypesJSONRequestBody{
			Filter: &ct.ProductResourceTypeFilter{Slugs: []string{"folder", "document"}},
		})

		assert.Equal(t, int64(2), found.Total)
		assert.ElementsMatch(t, []string{tree.folder.Id, tree.document.Id},
			[]string{found.Items[0].Id, found.Items[1].Id})
	})
}

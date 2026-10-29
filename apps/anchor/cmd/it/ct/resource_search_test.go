package ct_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"

	itdsl "anchor/cmd/it/shared/dsl"
)

// searchWorld registers a project, two folders in it, a document in one of
// them, and a project in another Organization that must never show.
type searchWorld struct {
	projectWorld
	design, specs, pricing string
}

func givenSearchWorld(t *testing.T) searchWorld {
	t.Helper()
	w := givenProject(t)
	s := searchWorld{projectWorld: w, design: resourceID(), specs: resourceID(), pricing: resourceID()}
	w.resources.Put("folder", s.design, "Design 100%", itdsl.Ref("project", w.project))
	w.resources.Put("folder", s.specs, "Specs", itdsl.Ref("project", w.project))
	w.resources.Put("document", s.pricing, "Pricing design", itdsl.Ref("folder", s.design))
	other := w.product.Resources(w.product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil).Id)
	other.Put("project", s.design, "Design elsewhere", other.Top())
	return s
}

func itemIDs(list ct.ResourceListResponse) []string {
	found := make([]string, len(list.Items))
	for i, item := range list.Items {
		found[i] = item.Id
	}
	return found
}

func TestSearchResources(t *testing.T) {
	t.Parallel()

	t.Run("A search without a filter answers every resource of the organization by creation", func(t *testing.T) {
		t.Parallel()
		s := givenSearchWorld(t)

		found := s.resources.Search(ct.SearchResourcesJSONRequestBody{})

		assert.Equal(t, []string{s.project, s.design, s.specs, s.pricing}, itemIDs(found))
		assert.Equal(t, int64(4), found.Total)
	})

	t.Run("A resource type filter keeps resources of those types", func(t *testing.T) {
		t.Parallel()
		s := givenSearchWorld(t)

		found := s.resources.Search(ct.SearchResourcesJSONRequestBody{
			Filter: &ct.ResourceFilter{ResourceTypes: []string{"folder"}},
		})

		assert.Equal(t, []string{s.design, s.specs}, itemIDs(found))
	})

	t.Run("A parent filter keeps the parent's direct children", func(t *testing.T) {
		t.Parallel()
		s := givenSearchWorld(t)
		parent := itdsl.Ref("project", s.project)

		found := s.resources.Search(ct.SearchResourcesJSONRequestBody{Filter: &ct.ResourceFilter{Parent: &parent}})

		assert.Equal(t, []string{s.design, s.specs}, itemIDs(found))
	})

	t.Run("The organization as parent filter keeps the top-level resources", func(t *testing.T) {
		t.Parallel()
		s := givenSearchWorld(t)
		top := s.resources.Top()

		found := s.resources.Search(ct.SearchResourcesJSONRequestBody{Filter: &ct.ResourceFilter{Parent: &top}})

		assert.Equal(t, []string{s.project}, itemIDs(found))
	})

	t.Run("A parent that is not registered matches nothing", func(t *testing.T) {
		t.Parallel()
		s := givenSearchWorld(t)
		parent := itdsl.Ref("project", resourceID())

		found := s.resources.Search(ct.SearchResourcesJSONRequestBody{Filter: &ct.ResourceFilter{Parent: &parent}})

		assert.Empty(t, found.Items)
	})

	t.Run("An ids filter keeps resources with those identifiers", func(t *testing.T) {
		t.Parallel()
		s := givenSearchWorld(t)

		found := s.resources.Search(ct.SearchResourcesJSONRequestBody{
			Filter: &ct.ResourceFilter{Ids: []string{s.pricing, s.design}},
		})

		assert.Equal(t, []string{s.design, s.pricing}, itemIDs(found))
	})

	t.Run("A name filter matches part of the name regardless of case", func(t *testing.T) {
		t.Parallel()
		s := givenSearchWorld(t)

		found := s.resources.Search(ct.SearchResourcesJSONRequestBody{
			Filter: &ct.ResourceFilter{NameContains: new("DESIGN")},
		})

		assert.Equal(
			t,
			[]string{s.project, s.design, s.pricing},
			itemIDs(found),
			"Website redesign, Design 100%, Pricing design",
		)
	})

	t.Run("A percent sign in the name filter matches itself only", func(t *testing.T) {
		t.Parallel()
		s := givenSearchWorld(t)

		found := s.resources.Search(ct.SearchResourcesJSONRequestBody{
			Filter: &ct.ResourceFilter{NameContains: new("0%")},
		})

		assert.Equal(t, []string{s.design}, itemIDs(found))
	})

	t.Run("Sorting by name orders the resources by name", func(t *testing.T) {
		t.Parallel()
		s := givenSearchWorld(t)

		found := s.resources.Search(ct.SearchResourcesJSONRequestBody{SortBy: new(ct.ResourceSortFieldName)})

		assert.Equal(t, []string{s.design, s.pricing, s.specs, s.project}, itemIDs(found))
	})

	t.Run("A search in an organization the product does not have answers 404", func(t *testing.T) {
		t.Parallel()
		s := givenSearchWorld(t)

		resp := s.product.Resources(ids.MustNew("org")).SearchRaw(ct.SearchResourcesJSONRequestBody{})

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "ORGANIZATION_NOT_FOUND")
	})
}

func TestSearchResourcesLimits(t *testing.T) {
	t.Parallel()

	repeated := func(n int, value func(int) string) []string {
		values := make([]string, n)
		for i := range values {
			values[i] = value(i)
		}
		return values
	}

	t.Run("An ids filter of 101 identifiers answers 400 VALIDATION_ERROR", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.SearchRaw(ct.SearchResourcesJSONRequestBody{Filter: &ct.ResourceFilter{
			Ids: repeated(101, func(int) string { return resourceID() }),
		}})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "VALIDATION_ERROR")
	})

	t.Run("A resource_types filter of 51 slugs answers 400 VALIDATION_ERROR", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.SearchRaw(ct.SearchResourcesJSONRequestBody{Filter: &ct.ResourceFilter{
			ResourceTypes: repeated(51, func(i int) string { return fmt.Sprintf("type_%d", i) }),
		}})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "VALIDATION_ERROR")
	})

	t.Run("A name_contains of 201 characters answers 400 VALIDATION_ERROR", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.SearchRaw(ct.SearchResourcesJSONRequestBody{Filter: &ct.ResourceFilter{
			NameContains: new(strings.Repeat("n", 201)),
		}})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "VALIDATION_ERROR")
	})

	t.Run("The largest filters the limits allow are accepted", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		found := w.resources.Search(ct.SearchResourcesJSONRequestBody{Filter: &ct.ResourceFilter{
			Ids:           repeated(100, func(int) string { return resourceID() }),
			ResourceTypes: repeated(50, func(i int) string { return fmt.Sprintf("type_%d", i) }),
			NameContains:  new(strings.Repeat("n", 200)),
		}})

		assert.Empty(t, found.Items)
	})
}

package ct_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

func TestPutResource(t *testing.T) {
	t.Parallel()

	t.Run(
		"Registering a resource under its organization answers it with the organization as parent",
		func(t *testing.T) {
			t.Parallel()
			w := givenResourceWorld(t)
			id := resourceID()

			resp := w.resources.PutRaw("project", id, "Website redesign", w.resources.Top())

			require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
			got := *resp.JSON200
			assert.Equal(t, "project", got.Type)
			assert.Equal(t, id, got.Id)
			assert.Equal(t, w.org.Id, got.OrganizationId)
			assert.Equal(t, "Website redesign", got.Name)
			assert.Equal(t, w.resources.Top(), got.Parent)
			assert.NotContains(t, string(resp.Body), resourceRowID(t, w.org.Id, id), "the row ID never leaves Anchor")
		},
	)

	t.Run("Registering a resource under a registered parent answers that parent", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		project := resourceID()
		w.resources.Put("project", project, "Website redesign", w.resources.Top())

		got := w.resources.Put("folder", resourceID(), "Design", itdsl.Ref("project", project))

		assert.Equal(t, itdsl.Ref("project", project), got.Parent)
	})

	t.Run("Registering the same resource twice leaves it unchanged", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		id := resourceID()
		first := w.resources.Put("project", id, "Website redesign", w.resources.Top())

		second := w.resources.Put("project", id, "Website redesign", w.resources.Top())

		assert.Equal(t, first, second)
		assert.Equal(t, 1, countResources(t, "external_id", id))
	})

	t.Run("Sending a new name renames the resource", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)

		renamed := w.resources.Put("project", w.project, "Relaunch", w.resources.Top())

		assert.Equal(t, "Relaunch", renamed.Name)
		assert.Equal(t, "Relaunch", w.resources.Get("project", w.project).Name)
	})

	t.Run("Sending a new parent moves the resource with its subtree", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		other := resourceID()
		w.resources.Put("project", other, "Other", w.resources.Top())
		design := resourceID()
		w.resources.Put("folder", design, "Design", itdsl.Ref("project", w.project))
		specs := resourceID()
		w.resources.Put("folder", specs, "Specs", itdsl.Ref("folder", design))

		moved := w.resources.Put("folder", design, "Design", itdsl.Ref("project", other))

		assert.Equal(t, itdsl.Ref("project", other), moved.Parent)
		assert.Equal(t, itdsl.Ref("folder", design), w.resources.Get("folder", specs).Parent)
	})

	t.Run("A folder sits under another folder because its type nests in itself", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		design := resourceID()
		w.resources.Put("folder", design, "Design", itdsl.Ref("project", w.project))

		nested := w.resources.Put("folder", resourceID(), "Specs", itdsl.Ref("folder", design))

		assert.Equal(t, itdsl.Ref("folder", design), nested.Parent)
	})

	t.Run("One identifier names different resources in another organization and another type", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		other := w.product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		otherResources := w.product.Resources(other.Id)
		id := resourceID()
		w.resources.Put("project", id, "Here", w.resources.Top())

		elsewhere := otherResources.Put("project", id, "Elsewhere", otherResources.Top())
		document := w.resources.Put("document", id, "Readme", itdsl.Ref("project", id))

		assert.Equal(t, "Elsewhere", elsewhere.Name)
		assert.Equal(t, "Readme", document.Name)
		assert.Equal(t, "Here", w.resources.Get("project", id).Name)
	})

	t.Run("A name of 200 characters is accepted", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		name := strings.Repeat("n", 200)

		got := w.resources.Put("project", resourceID(), name, w.resources.Top())

		assert.Equal(t, name, got.Name)
	})

	t.Run("An identifier of 200 characters from the allowed set is accepted", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		id := ids.MustNew("ext") + "-._~" + strings.Repeat("a", 200)
		id = id[:200]

		got := w.resources.Put("project", id, "Long", w.resources.Top())

		assert.Equal(t, id, got.Id)
	})
}

// projectWorld is a resource world with one project registered at the top.
type projectWorld struct {
	resourceWorld
	project string
}

func givenProject(t *testing.T) projectWorld {
	t.Helper()
	w := givenResourceWorld(t)
	project := resourceID()
	w.resources.Put("project", project, "Website redesign", w.resources.Top())
	return projectWorld{resourceWorld: w, project: project}
}

func TestPutResourceRefusals(t *testing.T) {
	t.Parallel()

	t.Run("A parent type the type does not list answers 400 INVALID_RESOURCE_PARENT", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.PutRaw("document", resourceID(), "Readme", w.resources.Top())

		meta := requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "INVALID_RESOURCE_PARENT")
		assert.Equal(t, []any{"folder", "project"}, meta["allowed_parent_types"])
	})

	t.Run(
		"A parent of its own type answers 400 INVALID_RESOURCE_PARENT when the type does not nest",
		func(t *testing.T) {
			t.Parallel()
			w := givenProject(t)

			resp := w.resources.PutRaw("project", resourceID(), "Sub", itdsl.Ref("project", w.project))

			requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "INVALID_RESOURCE_PARENT")
		},
	)

	t.Run("A type the product has not declared answers 404 RESOURCE_TYPE_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.PutRaw("spreadsheet", resourceID(), "Budget", w.resources.Top())

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_TYPE_NOT_FOUND")
	})

	t.Run("The organization type answers 400 BUILT_IN_RESOURCE_TYPE", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.PutRaw("organization", resourceID(), "Acme", w.resources.Top())

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "BUILT_IN_RESOURCE_TYPE")
	})

	t.Run("An organization the product does not have answers 404 ORGANIZATION_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		missing := w.product.Resources(ids.MustNew("org"))

		resp := missing.PutRaw("project", resourceID(), "Lost", missing.Top())

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "ORGANIZATION_NOT_FOUND")
	})

	t.Run("A parent that is not registered answers 409 PARENT_NOT_REGISTERED", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		parent := itdsl.Ref("project", resourceID())

		resp := w.resources.PutRaw("folder", resourceID(), "Design", parent)

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "PARENT_NOT_REGISTERED")
		assert.Equal(t, map[string]any{"type": "project", "id": parent.Id}, meta["parent"])
	})

	t.Run("A parent registered in another organization answers 409 PARENT_NOT_REGISTERED", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		other := w.product.Resources(w.product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil).Id)
		project := resourceID()
		other.Put("project", project, "Theirs", other.Top())

		resp := w.resources.PutRaw("folder", resourceID(), "Design", itdsl.Ref("project", project))

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "PARENT_NOT_REGISTERED")
	})

	t.Run("Another organization as parent answers 409 PARENT_NOT_REGISTERED", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		other := w.product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)

		resp := w.resources.PutRaw("project", resourceID(), "Stray", itdsl.Ref("organization", other.Id))

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "PARENT_NOT_REGISTERED")
	})

	t.Run("A resource moved under itself answers 409 RESOURCE_CYCLE", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		design := resourceID()
		w.resources.Put("folder", design, "Design", itdsl.Ref("project", w.project))

		resp := w.resources.PutRaw("folder", design, "Design", itdsl.Ref("folder", design))

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_CYCLE")
		assert.Equal(t, []any{"folder:" + design}, meta["chain"])
	})

	t.Run("A resource moved under its own descendant answers 409 RESOURCE_CYCLE with the chain", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		design := resourceID()
		w.resources.Put("folder", design, "Design", itdsl.Ref("project", w.project))
		specs := resourceID()
		w.resources.Put("folder", specs, "Specs", itdsl.Ref("folder", design))

		resp := w.resources.PutRaw("folder", design, "Design", itdsl.Ref("folder", specs))

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_CYCLE")
		assert.Equal(t, []any{"folder:" + specs, "folder:" + design}, meta["chain"])
		assert.Equal(t, itdsl.Ref("project", w.project), w.resources.Get("folder", design).Parent)
	})

	t.Run("A chain of 15 levels counting the organization is accepted", func(t *testing.T) {
		t.Parallel()
		w := givenNestingWorld(t)

		chain := chainOf(t, w, 14)

		assert.Equal(t, itdsl.Ref("node", chain[12]), w.resources.Get("node", chain[13]).Parent)
	})

	t.Run("A sixteenth level answers 409 RESOURCE_TOO_DEEP", func(t *testing.T) {
		t.Parallel()
		w := givenNestingWorld(t)
		chain := chainOf(t, w, 14)

		resp := w.resources.PutRaw("node", resourceID(), "Too deep", itdsl.Ref("node", chain[13]))

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TOO_DEEP")
		assert.EqualValues(t, 15, meta["max_depth"])
		assert.EqualValues(t, 16, meta["depth"])
	})

	t.Run("A move that puts a descendant at a sixteenth level answers 409 RESOURCE_TOO_DEEP", func(t *testing.T) {
		t.Parallel()
		w := givenNestingWorld(t)
		chain := chainOf(t, w, 13)
		branch := resourceID()
		w.resources.Put("node", branch, "Branch", w.resources.Top())
		w.resources.Put("node", resourceID(), "Leaf", itdsl.Ref("node", branch))

		resp := w.resources.PutRaw("node", branch, "Branch", itdsl.Ref("node", chain[12]))

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TOO_DEEP")
		assert.EqualValues(t, 16, meta["depth"])
		assert.Equal(t, w.resources.Top(), w.resources.Get("node", branch).Parent)
	})

	t.Run("An identifier with a character outside the allowed set answers 400 INVALID_RESOURCE_ID", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.PutRaw("project", "a+b", "Plus", w.resources.Top())

		meta := requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "INVALID_RESOURCE_ID")
		assert.Equal(t, "a+b", meta["id"])
	})

	t.Run("An identifier of 201 characters answers 400 INVALID_RESOURCE_ID", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.PutRaw("project", strings.Repeat("a", 201), "Long", w.resources.Top())

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "INVALID_RESOURCE_ID")
	})

	t.Run("A parent identifier outside the allowed set answers 400 INVALID_RESOURCE_ID", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.PutRaw("folder", resourceID(), "Design", itdsl.Ref("project", "a+b"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "INVALID_RESOURCE_ID")
	})

	t.Run("A name of 201 characters answers 400 VALIDATION_ERROR", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.PutRaw("project", resourceID(), strings.Repeat("n", 201), w.resources.Top())

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "VALIDATION_ERROR")
	})

	t.Run("A body without a parent answers 400 and never defaults to the organization", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		id := resourceID()

		resp, err := w.product.AllScopeAPIKeyClient().PutResourceWithBodyWithResponse(
			t.Context(), w.product.ProductID, w.org.Id, "project", id,
			"application/json", bytes.NewReader([]byte(`{"name":"Orphan"}`)),
		)
		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		read := w.resources.GetRaw("project", id)
		requireAPIError(t, read, read.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})
}

func TestGetResource(t *testing.T) {
	t.Parallel()

	t.Run("Reading a registered resource answers it", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)

		got := w.resources.Get("project", w.project)

		assert.Equal(t, "Website redesign", got.Name)
	})

	t.Run("Reading an identifier nothing registered answers 404 RESOURCE_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.GetRaw("project", resourceID())

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})

	t.Run("Reading a resource through another organization answers 404 RESOURCE_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		other := w.product.Resources(w.product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil).Id)

		resp := other.GetRaw("project", w.project)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})

	t.Run("Reading a resource under its other type answers 404 RESOURCE_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)

		resp := w.resources.GetRaw("folder", w.project)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})
}

func TestDeleteResource(t *testing.T) {
	t.Parallel()

	t.Run("Deleting a resource without children removes it", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)

		w.resources.Delete("project", w.project, false)

		resp := w.resources.GetRaw("project", w.project)
		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})

	t.Run("Deleting a resource with children answers 409 RESOURCE_IN_USE with the counts", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		w.resources.Put("folder", resourceID(), "Design", itdsl.Ref("project", w.project))
		w.resources.Put("document", resourceID(), "Readme", itdsl.Ref("project", w.project))

		resp := w.resources.DeleteRaw("project", w.project, false)

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_IN_USE")
		assert.EqualValues(t, 2, meta["children"])
		assert.EqualValues(t, 0, meta["role_assignments"])
		w.resources.Get("project", w.project)
	})

	t.Run("Deleting with cascade removes the resource and its whole subtree", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		design := resourceID()
		w.resources.Put("folder", design, "Design", itdsl.Ref("project", w.project))
		specs := resourceID()
		w.resources.Put("folder", specs, "Specs", itdsl.Ref("folder", design))
		pricing := resourceID()
		w.resources.Put("document", pricing, "Pricing", itdsl.Ref("folder", specs))

		w.resources.Delete("folder", design, true)

		for _, gone := range []ct.ResourceParentRef{
			itdsl.Ref("folder", design), itdsl.Ref("folder", specs), itdsl.Ref("document", pricing),
		} {
			resp := w.resources.GetRaw(gone.Type, gone.Id)
			requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
		}
		w.resources.Get("project", w.project)
	})

	t.Run("Deleting an identifier nothing registered answers 404 RESOURCE_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)

		resp := w.resources.DeleteRaw("project", resourceID(), true)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})
}

func TestDeleteOwnersOfResources(t *testing.T) {
	t.Parallel()

	t.Run("Deleting an organization removes its nested resources", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		design := resourceID()
		w.resources.Put("folder", design, "Design", itdsl.Ref("project", w.project))
		w.resources.Put("folder", resourceID(), "Specs", itdsl.Ref("folder", design))

		resp, err := w.product.AllScopeAPIKeyClient().DeleteProductOrganizationWithResponse(
			t.Context(), w.product.ProductID, w.org.Id)
		require.NoError(t, err)

		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
		assert.Zero(t, countResources(t, "organization_id", w.org.Id))
	})

	t.Run("Deleting a product removes its nested resources", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		design := resourceID()
		w.resources.Put("folder", design, "Design", itdsl.Ref("project", w.project))
		w.resources.Put("document", resourceID(), "Readme", itdsl.Ref("folder", design))

		resp, err := w.product.OwnerAuthenticatedClient().DeleteProductWithResponse(t.Context(), w.product.ProductID)
		require.NoError(t, err)

		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
		assert.Zero(t, countResources(t, "product_id", w.product.ProductID))
	})
}

func TestResourceScopes(t *testing.T) {
	t.Parallel()

	t.Run("Registering needs resource:write", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		reader := w.product.Resources(w.org.Id, "resource:read", "resource:delete")
		writer := w.product.Resources(w.org.Id, "resource:write")

		refused := reader.PutRaw("project", resourceID(), "Refused", reader.Top())
		accepted := writer.PutRaw("project", resourceID(), "Accepted", writer.Top())

		assert.Equal(t, http.StatusForbidden, refused.StatusCode(), string(refused.Body))
		assert.Equal(t, http.StatusOK, accepted.StatusCode(), string(accepted.Body))
	})

	t.Run("Reading and searching need resource:read", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		writer := w.product.Resources(w.org.Id, "resource:write", "resource:delete")
		reader := w.product.Resources(w.org.Id, "resource:read")

		assert.Equal(t, http.StatusForbidden, writer.GetRaw("project", w.project).StatusCode())
		assert.Equal(t, http.StatusForbidden, writer.SearchRaw(ct.SearchResourcesJSONRequestBody{}).StatusCode())
		assert.Equal(t, http.StatusOK, reader.GetRaw("project", w.project).StatusCode())
		assert.Equal(t, http.StatusOK, reader.SearchRaw(ct.SearchResourcesJSONRequestBody{}).StatusCode())
	})

	t.Run("Deleting needs resource:delete", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		writer := w.product.Resources(w.org.Id, "resource:read", "resource:write")
		deleter := w.product.Resources(w.org.Id, "resource:delete")

		assert.Equal(t, http.StatusForbidden, writer.DeleteRaw("project", w.project, false).StatusCode())
		assert.Equal(t, http.StatusNoContent, deleter.DeleteRaw("project", w.project, false).StatusCode())
	})
}

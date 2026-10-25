package ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/internal/authz/model"
)

const (
	mirrorWait = 15 * time.Second
	mirrorPoll = 50 * time.Millisecond
)

func TestResourceEvents(t *testing.T) {
	t.Parallel()

	t.Run("Resource events move the relationship store and never reach the event endpoint", func(t *testing.T) {
		t.Parallel()
		w := givenProject(t)
		sink := w.product.CaptureEvents()
		projectObject := model.ResourceObject("project", resourceRowID(t, w.org.Id, w.project))
		design := resourceID()

		w.resources.Put("folder", design, "Design", itdsl.Ref("project", w.project))
		designObject := model.ResourceObject("folder", resourceRowID(t, w.org.Id, design))
		require.Eventually(t, func() bool {
			parents := storedParents(t, w.product.ProductID, designObject)
			return len(parents) == 1 && parents[0] == projectObject
		}, mirrorWait, mirrorPoll, "resource.created syncs the parent tuple")

		other := resourceID()
		w.resources.Put("project", other, "Other", w.resources.Top())
		otherObject := model.ResourceObject("project", resourceRowID(t, w.org.Id, other))
		w.resources.Put("folder", design, "Design", itdsl.Ref("project", other))
		require.Eventually(t, func() bool {
			parents := storedParents(t, w.product.ProductID, designObject)
			return len(parents) == 1 && parents[0] == otherObject
		}, mirrorWait, mirrorPoll, "resource.updated replaces the parent tuple")
		assert.Equal(t, []string{model.Organization(w.org.Id)}, storedParents(t, w.product.ProductID, projectObject))

		w.resources.Delete("folder", design, false)
		renamed, err := w.product.AllScopeAPIKeyClient().UpdateProductOrganizationWithResponse(
			t.Context(), w.product.ProductID, w.org.Id, ct.UpdateProductOrganizationJSONRequestBody{Name: "Renamed"})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, renamed.StatusCode(), string(renamed.Body))

		sink.WaitFor("organization.updated", map[string]string{"organization_id": w.org.Id})
		for _, eventType := range []string{"resource.created", "resource.updated", "resource.deleted"} {
			assert.Zero(t, sink.Count(eventType), eventType)
		}
	})

	t.Run("An endpoint can't subscribe to resource events", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		owner := product.OwnerAuthenticatedClient()
		got, err := owner.GetProductWithResponse(t.Context(), product.ProductID)
		require.NoError(t, err)
		require.NotNil(t, got.JSON200, string(got.Body))
		endpoint := "http://127.0.0.1:9/events"

		resp, err := owner.UpdateProductWithResponse(t.Context(), product.ProductID, ct.UpdateProductJSONRequestBody{
			Name:        got.JSON200.Name,
			Description: got.JSON200.Description,
			Config: &ct.ProductConfigRequest{
				OrganizationApiKeys: &ct.ProductOrganizationAPIKeysConfigRequest{
					Prefix: got.JSON200.Config.OrganizationApiKeys.Prefix,
				},
				Events: &ct.ProductEventsConfigRequest{EndpointUrl: &endpoint, Events: &[]string{"resource.created"}},
			},
		})
		require.NoError(t, err)

		meta := requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "UNKNOWN_EVENT_TYPE")
		assert.Equal(t, "resource.created", meta["event_type"])
	})

	t.Run("The product's event catalog does not list resource events", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp, err := product.OwnerAuthenticatedClient().
			GetProductEventsCatalogWithResponse(t.Context(), product.ProductID)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotEmpty(t, resp.JSON200.Items)
		for _, definition := range resp.JSON200.Items {
			assert.NotContains(t, []string{"resource.created", "resource.updated", "resource.deleted"}, definition.Type)
		}
	})
}

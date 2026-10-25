package ct_test

import (
	"context"
	"net/http"
	"sync"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

const (
	concurrentRounds  = 5
	concurrentWriters = 8
)

func TestConcurrentResourceWrites(t *testing.T) {
	t.Parallel()

	t.Run("Two moves that would form a cycle never both succeed", func(t *testing.T) {
		t.Parallel()
		w := givenNestingWorld(t)
		for range concurrentRounds {
			a, b := resourceID(), resourceID()
			w.resources.Put("node", a, "A", w.resources.Top())
			w.resources.Put("node", b, "B", w.resources.Top())

			var wg sync.WaitGroup
			statuses := make([]int, 2)
			moves := [][2]string{{a, b}, {b, a}}
			for i, move := range moves {
				wg.Go(func() {
					statuses[i] = w.resources.PutRaw("node", move[0], "Moved", itdsl.Ref("node", move[1])).StatusCode()
				})
			}
			wg.Wait()

			assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, statuses)
			parentOfA, parentOfB := w.resources.Get("node", a).Parent, w.resources.Get("node", b).Parent
			assert.False(t, parentOfA == itdsl.Ref("node", b) && parentOfB == itdsl.Ref("node", a),
				"A and B sit under each other")
		}
	})

	t.Run("Concurrent registrations of one resource all succeed and leave one resource", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		for range concurrentRounds {
			id := resourceID()
			var wg sync.WaitGroup
			statuses := make([]int, concurrentWriters)
			for i := range concurrentWriters {
				wg.Go(func() {
					resp := w.resources.PutRaw("project", id, "Same", w.resources.Top())
					statuses[i] = resp.StatusCode()
				})
			}
			wg.Wait()

			for _, status := range statuses {
				require.Equal(t, http.StatusOK, status)
			}
			assert.Equal(t, 1, countResources(t, "external_id", id))
		}
	})

	t.Run(
		"A registration racing the removal of its parent type never leaves the resource under it",
		func(t *testing.T) {
			t.Parallel()
			for range concurrentRounds {
				w := givenProject(t)
				var wg sync.WaitGroup
				var putStatus, patchStatus int
				wg.Go(func() {
					putStatus = w.resources.PutRaw("document", resourceID(), "Readme",
						itdsl.Ref("project", w.project)).StatusCode()
				})
				wg.Go(func() {
					patchStatus = w.product.ResourceTypes().UpdateRaw(w.tree.document.Id,
						ct.UpdateProductResourceTypeJSONRequestBody{ParentTypes: &[]string{"folder"}}).StatusCode()
				})
				wg.Wait()

				assert.Contains(t, [][2]int{
					{http.StatusOK, http.StatusConflict},
					{http.StatusBadRequest, http.StatusOK},
				}, [2]int{putStatus, patchStatus}, "registration then removal, or removal then registration")
			}
		},
	)

	t.Run("Registrations racing their organization's delete answer 200 or 404, never 500", func(t *testing.T) {
		t.Parallel()
		w := givenResourceWorld(t)
		for range concurrentRounds {
			org := w.product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
			resources := w.product.Resources(org.Id)
			project := resourceID()
			resources.Put("project", project, "Project", resources.Top())

			var wg sync.WaitGroup
			statuses := make([]int, concurrentWriters)
			for i := range concurrentWriters {
				wg.Go(func() {
					statuses[i] = resources.PutRaw("folder", resourceID(), "Folder",
						itdsl.Ref("project", project)).StatusCode()
				})
			}
			wg.Go(func() {
				resp, err := w.product.AllScopeAPIKeyClient().DeleteProductOrganizationWithResponse(
					context.Background(), w.product.ProductID, org.Id)
				if assert.NoError(t, err) {
					assert.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
				}
			})
			wg.Wait()

			for _, status := range statuses {
				assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, status)
			}
			assert.Zero(t, countResources(t, "organization_id", org.Id))
		}
	})
}

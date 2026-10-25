package ct_test

import (
	"net/http"
	"sync"
	"testing"

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
}

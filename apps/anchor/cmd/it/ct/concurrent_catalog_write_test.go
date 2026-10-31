package ct_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
)

func TestConcurrentCatalogWritesOfOneProductAllSucceed(t *testing.T) {
	t.Parallel()

	t.Run("Eight permissions with different names created at once are all created", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		client := product.OwnerAuthenticatedClient()

		for round := range 5 {
			var wg sync.WaitGroup
			statuses := make([]int, 8)
			bodies := make([]string, 8)
			for writer := range 8 {
				wg.Go(func() {
					resp, err := client.CreateProductResourcePermissionWithResponse(t.Context(), product.ProductID,
						ct.CreateProductResourcePermissionRequest{
							ResourceType: "organization",
							Name:         fmt.Sprintf("concurrent:r%d_w%d", round, writer),
						})
					if err != nil {
						bodies[writer] = err.Error()
						return
					}
					statuses[writer] = resp.StatusCode()
					bodies[writer] = string(resp.Body)
				})
			}
			wg.Wait()

			for writer, status := range statuses {
				assert.Equal(t, http.StatusCreated, status, "round %d writer %d: %s", round, writer, bodies[writer])
			}
		}
	})
}

package ct_test

import (
	"net/http"
	"sync"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A product the previous release created has no organization type. These
// tests put such products in place with SQL scoped to their own product.

func TestProductWithoutOrganizationTypeGetsOne(t *testing.T) {
	t.Parallel()
	t.Run("A product the previous release created gets its organization type on first read", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		execSQL(t, "DELETE FROM product_resource_types WHERE product_id = $1", product.ProductID)

		found := product.ResourceTypes().Search(ct.SearchProductResourceTypesJSONRequestBody{})

		require.Len(t, found.Items, 1)
		assert.Equal(t, "organization", found.Items[0].Slug)
		assert.True(t, found.Items[0].BuiltIn)
	})
}

// A role's permission scopes check and a tree change race on the catalog lock:
// one of them must lose, or a project role could end up holding a document permission
// that is no longer below project.
func TestConcurrentRolePermissionScopesAndTreeChange(t *testing.T) {
	t.Parallel()
	t.Run(
		"A project role holding a document permission and cutting document off from project never both succeed",
		func(t *testing.T) {
			t.Parallel()
			const rounds = 20
			for range rounds {
				product := createTestProductContext(t)
				tree := declareDocumentTree(t, product)
				types := product.ResourceTypes()
				types.Update(
					tree.document.Id,
					ct.UpdateProductResourceTypeJSONRequestBody{ParentTypes: &[]string{"folder"}},
				)
				product.CreateScopedPermissions(t, "document", "document:view")
				client := product.AllScopeAPIKeyClient()

				var roleStatus, treeStatus int
				var wg sync.WaitGroup
				start := make(chan struct{})
				wg.Go(func() {
					<-start
					resp, err := client.CreateProductRoleWithResponse(t.Context(), product.ProductID,
						ct.CreateProductRoleJSONRequestBody{
							Name: "project-reader", ResourceType: "project", Permissions: []string{"document:view"},
						})
					if assert.NoError(t, err) {
						roleStatus = resp.StatusCode()
					}
				})
				wg.Go(func() {
					<-start
					resp, err := client.UpdateProductResourceTypeWithResponse(
						t.Context(),
						product.ProductID,
						tree.folder.Id,
						ct.UpdateProductResourceTypeJSONRequestBody{ParentTypes: &[]string{"organization"}},
					)
					if assert.NoError(t, err) {
						treeStatus = resp.StatusCode()
					}
				})
				close(start)
				wg.Wait()

				assert.False(t, roleStatus == http.StatusCreated && treeStatus == http.StatusOK,
					"role create %d and tree change %d both succeeded", roleStatus, treeStatus)
				assert.Contains(t, []int{http.StatusCreated, http.StatusBadRequest}, roleStatus)
				assert.Contains(t, []int{http.StatusOK, http.StatusConflict}, treeStatus)
			}
		},
	)
}

func TestProductWithoutOrganizationTypeAcceptsAWrite(t *testing.T) {
	t.Parallel()
	t.Run(
		"A product the previous release created gets its organization type on its first permission",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			execSQL(t, "DELETE FROM product_resource_types WHERE product_id = $1", product.ProductID)

			created := product.CreateProductResourcePermissions(t, "legacy:write")[0]

			assert.Equal(t, "organization", created.ResourceType)
		},
	)
}

package ct_test

import (
	"net/http"
	"sync"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// During a rolling deploy the previous release writes roles and permissions
// with no scope, and products with no organization type. These tests put such
// rows in place with SQL scoped to their own product.

func TestUnscopedPermissionReadsAsOrganization(t *testing.T) {
	t.Parallel()
	t.Run("A permission with no scope reads, and filters, as organization", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		product.CreateProductResourcePermissions(t, "legacy:read")
		execSQL(t, "UPDATE product_resource_permissions SET resource_type_id = NULL WHERE product_id = $1",
			product.ProductID)
		client := product.OwnerAuthenticatedClient()

		read, err := client.GetProductResourcePermissionWithResponse(t.Context(), product.ProductID, "legacy:read")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, read.StatusCode())
		found, err := client.SearchProductResourcePermissionsWithResponse(t.Context(), product.ProductID,
			ct.SearchProductResourcePermissionsJSONRequestBody{
				Filter: &ct.ProductResourcePermissionFilter{ResourceTypes: []string{"organization"}},
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, found.StatusCode())

		assert.Equal(t, "organization", read.JSON200.ResourceType)
		require.Len(t, found.JSON200.Items, 1)
		assert.Equal(t, "legacy:read", found.JSON200.Items[0].Name)
	})
}

func TestUnscopedRoleReadsAsOrganization(t *testing.T) {
	t.Parallel()
	t.Run("A role with no scope reads as organization and bundles any permission", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)
		product.CreateScopedPermissions(t, "document", "document:view")
		role := createScopedRole(t, product, "organization")
		execSQL(t, "UPDATE product_roles SET resource_type_id = NULL WHERE product_id = $1 AND id = $2",
			product.ProductID, role.Id)
		client := product.OwnerAuthenticatedClient()

		assigned, err := client.AssignPermissionToProductRoleWithResponse(t.Context(), product.ProductID, role.Id,
			ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: "document:view"})
		require.NoError(t, err)
		read, err := client.GetProductRoleWithResponse(t.Context(), product.ProductID, role.Id)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNoContent, assigned.StatusCode(), string(assigned.Body))
		require.Equal(t, http.StatusOK, read.StatusCode())
		assert.Equal(t, "organization", read.JSON200.ResourceType)
	})
}

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

// A role bundle check and a tree change race on the catalog lock: one of them
// must lose, or a project role could end up bundling a document permission
// that is no longer below project.
func TestConcurrentRoleBundleAndTreeChange(t *testing.T) {
	t.Parallel()
	t.Run(
		"A project role bundling a document permission and cutting document off from project never both succeed",
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

package ct_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/internal/authz/model"
)

// modelFillPermissions is enough permissions that about 33 resource types
// holding or inheriting them take the model past model.MaxBytes.
const modelFillPermissions = 160

func createModelFillPermissions(t *testing.T, product *itdsl.ProductContext, scope string) []string {
	t.Helper()
	names := make([]string, modelFillPermissions)
	for i := range names {
		names[i] = fmt.Sprintf("document:permission_with_a_long_readable_name_%03d", i)
	}
	product.CreateScopedPermissions(t, scope, names...)
	return names
}

func requireModelTooLarge(t *testing.T, resp statusResponse, body []byte) {
	t.Helper()
	metadata := requireAPIError(t, resp, body, http.StatusConflict, "AUTHZ_MODEL_TOO_LARGE")
	assert.InDelta(t, model.MaxBytes, metadata["max_model_bytes"], 0)
	assert.Greater(t, metadata["model_bytes"], float64(model.MaxBytes))
}

func resourceTypeSlugs(product *itdsl.ProductContext) []string {
	found := product.ResourceTypes().Search(ct.SearchProductResourceTypesJSONRequestBody{})
	slugs := make([]string, 0, len(found.Items))
	for _, rt := range found.Items {
		slugs = append(slugs, rt.Slug)
	}
	return slugs
}

func TestReplaceResourceTypeTreePastAuthzModelLimit(t *testing.T) {
	t.Parallel()
	t.Run("A tree whose model would pass the size limit answers 409 and changes nothing", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		createModelFillPermissions(t, product, "organization")
		items := make([]ct.ProductResourceTypeTreeItem, 50)
		for i := range items {
			items[i] = treeItem(fmt.Sprintf("type_%02d", i), "organization")
		}

		resp := product.ResourceTypes().ReplaceTreeRaw(items...)

		requireModelTooLarge(t, resp, resp.Body)
		assert.Equal(t, []string{"organization"}, resourceTypeSlugs(product))
	})
}

func TestCatalogWritesAtAuthzModelLimit(t *testing.T) {
	t.Parallel()
	product := createTestProductContext(t)
	permissions := createModelFillPermissions(t, product, "organization")
	owner := product.OwnerAuthenticatedClient()
	var created []ct.ProductResourceTypeResponse

	t.Run(
		"Creating types one by one stops with 409 at the size limit, and the refused type does not exist",
		func(t *testing.T) {
			refused := ""
			for i := range 50 {
				slug := fmt.Sprintf("type_%02d", i)
				resp := product.ResourceTypes().CreateRaw(ct.CreateProductResourceTypeJSONRequestBody{
					Slug: slug, Name: slug, ParentTypes: []string{"organization"},
				})
				if resp.StatusCode() == http.StatusConflict {
					requireModelTooLarge(t, resp, resp.Body)
					refused = slug
					break
				}
				require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
				created = append(created, *resp.JSON201)
			}

			require.NotEmpty(t, refused, "no type create reached the size limit")
			assert.NotContains(t, resourceTypeSlugs(product), refused)
		},
	)

	t.Run(
		"Creating a permission past the size limit answers 409, and the permission does not exist",
		func(t *testing.T) {
			refused := ""
			for i := range 10 {
				name := fmt.Sprintf("document:past_the_limit_%02d", i)
				resp, err := owner.CreateProductResourcePermissionWithResponse(context.Background(), product.ProductID,
					ct.CreateProductResourcePermissionJSONRequestBody{ResourceType: "organization", Name: name})
				require.NoError(t, err)
				if resp.StatusCode() == http.StatusConflict {
					requireModelTooLarge(t, resp, resp.Body)
					refused = name
					break
				}
				require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
			}

			require.NotEmpty(t, refused, "no permission create reached the size limit")
			got, err := owner.GetProductResourcePermissionWithResponse(context.Background(), product.ProductID, refused)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, got.StatusCode(), string(got.Body))
		},
	)

	t.Run("At the size limit, a description change and deletes still succeed", func(t *testing.T) {
		updated, err := owner.UpdateProductResourcePermissionWithResponse(context.Background(), product.ProductID,
			permissions[0], ct.UpdateProductResourcePermissionJSONRequestBody{Description: new("Still editable.")})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, updated.StatusCode(), string(updated.Body))

		deleted, err := owner.DeleteProductResourcePermissionWithResponse(context.Background(), product.ProductID,
			permissions[1])
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, deleted.StatusCode(), string(deleted.Body))

		last := created[len(created)-1]
		product.ResourceTypes().Delete(last.Id)
		assert.NotContains(t, resourceTypeSlugs(product), last.Slug)
	})
}

func TestUpdateResourceTypePastAuthzModelLimit(t *testing.T) {
	t.Parallel()
	t.Run(
		"Moving types under a type with many permissions stops with 409 at the size limit and keeps the refused type's parents",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			types := product.ResourceTypes()
			items := []ct.ProductResourceTypeTreeItem{treeItem("app", "organization")}
			for i := range 49 {
				items = append(items, treeItem(fmt.Sprintf("type_%02d", i), "organization"))
			}
			tree := types.ReplaceTree(items...)
			createModelFillPermissions(t, product, "app")

			refused := ""
			for _, rt := range tree {
				if rt.Slug == "organization" || rt.Slug == "app" {
					continue
				}
				resp := types.UpdateRaw(
					rt.Id,
					ct.UpdateProductResourceTypeJSONRequestBody{ParentTypes: &[]string{"app"}},
				)
				if resp.StatusCode() == http.StatusConflict {
					requireModelTooLarge(t, resp, resp.Body)
					refused = rt.Id
					break
				}
				require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
			}

			require.NotEmpty(t, refused, "no type update reached the size limit")
			assert.Equal(t, []string{"organization"}, types.Get(refused).ParentTypes)
		},
	)
}

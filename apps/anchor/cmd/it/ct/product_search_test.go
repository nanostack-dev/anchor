package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/nanostack-dev/shared/toolkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductSearch(t *testing.T) {
	ctx := t.Context()
	productsCreated := make([]ct.ProductResponse, 0)
	for range 5 {
		productName := "Test Product " + toolkit.NewID("prd")
		createResp, err := testOwnerClient(t).CreateProductWithResponse(
			ctx,
			ct.CreateProductJSONRequestBody{
				Name:        productName,
				Description: toolkit.Ptr("This is a test product " + productName),
			},
		)
		require.NoError(t, err, "create product request should not error")
		assert.Equal(
			t, http.StatusCreated,
			createResp.StatusCode(), "create product should return 201 Created",
		)
		assert.NotNil(t, createResp.JSON201)

		productsCreated = append(productsCreated, *createResp.JSON201)
	}

	t.Run(
		"Search with multiple search param", func(t *testing.T) {
			response, err := testOwnerClient(t).SearchProductsWithResponse(
				ctx,
				ct.SearchProductsJSONRequestBody{
					Filter: &ct.ProductFilter{
						Names: []string{
							productsCreated[0].Name,
							productsCreated[3].Name,
						},
					},
					Pagination: &ct.PaginationRequest{
						Limit:  toolkit.Ptr(int32(10)),
						Offset: toolkit.Ptr(int32(0)),
					},
				},
			)
			require.NoError(t, err, "search products request should not error")
			assert.Equal(t, 200, response.StatusCode(), "search products should return 200 OK")
			assert.NotNil(t, response.JSON200)
			assert.Len(t, response.JSON200.Items, 2, "should find 2 products")
			assert.Equal(
				t, []ct.ProductResponse{
					productsCreated[0],
					productsCreated[3],
				}, response.JSON200.Items, "should find the correct products",
			)

			response, err = testOwnerClient(t).SearchProductsWithResponse(
				ctx, ct.SearchProductsJSONRequestBody{
					Filter: &ct.ProductFilter{
						Ids: []string{
							productsCreated[0].Id,
							productsCreated[4].Id,
						},
					},
					Pagination: &ct.PaginationRequest{
						Limit:  toolkit.Ptr(int32(10)),
						Offset: toolkit.Ptr(int32(0)),
					},
				},
			)
			require.NoError(t, err, "search products by IDs request should not error")
			assert.Equal(
				t, 200, response.StatusCode(), "search products by IDs should return 200 OK",
			)
			assert.NotNil(t, response.JSON200)
			assert.Len(t, response.JSON200.Items, 2, "should find 2 products by IDs")
			assert.Equal(
				t, []ct.ProductResponse{
					productsCreated[0],
					productsCreated[4],
				}, response.JSON200.Items, "should find the correct products by IDs",
			)
		},
	)
}

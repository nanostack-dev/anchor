package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/nanostack-dev/shared/toolkit"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductAPIKeySearch(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	permission1 := "organization:read"
	permission2 := "organization:create"

	apiKey1Name := "TestKey1_" + uuid.NewString()
	apiKey2Name := "TestKey2_" + uuid.NewString()
	apiKey3Name := "SearchKey_" + uuid.NewString()

	createResp1, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
		ctx, product.ProductID,
		ct.CreateProductAPIKeyJSONRequestBody{
			Name:        apiKey1Name,
			Description: toolkit.Ptr("First test API key"),
			Permissions: []string{permission1},
		},
	)
	require.NoError(t, err)
	require.Equal(
		t, http.StatusCreated,
		createResp1.StatusCode(),
	)
	require.NotNil(t, createResp1.JSON201)

	createResp2, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
		ctx, product.ProductID,
		ct.CreateProductAPIKeyJSONRequestBody{
			Name:        apiKey2Name,
			Description: toolkit.Ptr("Second test API key"),
			Permissions: []string{permission2},
		},
	)
	require.NoError(t, err)
	require.Equal(
		t, http.StatusCreated,
		createResp2.StatusCode(),
	)
	require.NotNil(t, createResp2.JSON201)

	createResp3, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
		ctx, product.ProductID,
		ct.CreateProductAPIKeyJSONRequestBody{
			Name:        apiKey3Name,
			Description: toolkit.Ptr("Third test API key for search"),
			Permissions: []string{permission1, permission2},
		},
	)
	require.NoError(t, err)
	require.Equal(
		t, http.StatusCreated,
		createResp3.StatusCode(),
	)
	require.NotNil(t, createResp3.JSON201)

	t.Run(
		"Search all API keys", func(t *testing.T) {
			searchResp, searchErr := product.OwnerAuthenticatedClient().SearchProductAPIKeysWithResponse(
				ctx, product.ProductID, ct.SearchProductAPIKeysJSONRequestBody{},
			)
			require.NoError(t, searchErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.GreaterOrEqual(t, len(searchResp.JSON200.Items), 3)
			assert.GreaterOrEqual(t, searchResp.JSON200.Total, int64(3))
		},
	)

	t.Run(
		"Search API keys by names", func(t *testing.T) {
			searchResp, filterErr := product.OwnerAuthenticatedClient().SearchProductAPIKeysWithResponse(
				ctx, product.ProductID, ct.SearchProductAPIKeysJSONRequestBody{
					Filter: &ct.ProductAPIKeyFilter{
						Names: []string{apiKey1Name, apiKey3Name},
					},
				},
			)
			require.NoError(t, filterErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.Len(t, searchResp.JSON200.Items, 2)

			foundNames := make(map[string]bool)
			for _, apiKey := range searchResp.JSON200.Items {
				foundNames[apiKey.Name] = true
			}
			assert.True(t, foundNames[apiKey1Name])
			assert.True(t, foundNames[apiKey3Name])
			assert.False(t, foundNames[apiKey2Name])
		},
	)

	t.Run(
		"Search API keys by IDs", func(t *testing.T) {
			targetIDs := []string{createResp1.JSON201.Id, createResp2.JSON201.Id}
			searchResp, idsErr := product.OwnerAuthenticatedClient().SearchProductAPIKeysWithResponse(
				ctx, product.ProductID, ct.SearchProductAPIKeysJSONRequestBody{
					Filter: &ct.ProductAPIKeyFilter{
						Ids: targetIDs,
					},
				},
			)
			require.NoError(t, idsErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.Len(t, searchResp.JSON200.Items, 2)

			foundIDs := make(map[string]bool)
			for _, apiKey := range searchResp.JSON200.Items {
				foundIDs[apiKey.Id] = true
			}
			assert.True(t, foundIDs[createResp1.JSON201.Id])
			assert.True(t, foundIDs[createResp2.JSON201.Id])
		},
	)

	t.Run(
		"Search API keys by status", func(t *testing.T) {
			searchResp, statusErr := product.OwnerAuthenticatedClient().SearchProductAPIKeysWithResponse(
				ctx, product.ProductID, ct.SearchProductAPIKeysJSONRequestBody{
					Filter: &ct.ProductAPIKeyFilter{
						Status: &[]ct.ProductAPIKeyStatus{ct.ProductAPIKeyStatusACTIVE},
					},
				},
			)
			require.NoError(t, statusErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.GreaterOrEqual(t, len(searchResp.JSON200.Items), 3)

			for _, apiKey := range searchResp.JSON200.Items {
				assert.Equal(t, ct.ProductAPIKeyStatusACTIVE, apiKey.Status)
			}
		},
	)

	t.Run(
		"Search API keys with pagination", func(t *testing.T) {
			limit := int32(2)
			offset := int32(0)
			pagination := ct.PaginationRequest{
				Limit:  &limit,
				Offset: &offset,
			}
			searchResp, paginationErr := product.OwnerAuthenticatedClient().SearchProductAPIKeysWithResponse(
				ctx, product.ProductID, ct.SearchProductAPIKeysJSONRequestBody{
					Pagination: &pagination,
				},
			)
			require.NoError(t, paginationErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.Len(t, searchResp.JSON200.Items, 2)

			foundIDs := make(map[string]struct{}, len(searchResp.JSON200.Items))
			for _, apiKey := range searchResp.JSON200.Items {
				foundIDs[apiKey.Id] = struct{}{}
			}
			assert.Len(t, foundIDs, 2)
		},
	)

	t.Run(
		"Search API keys with sorting", func(t *testing.T) {
			sortBy := ct.ProductAPIKeySearchRequestSortByName
			sortDirection := ct.ASC
			searchResp, sortErr := product.OwnerAuthenticatedClient().SearchProductAPIKeysWithResponse(
				ctx, product.ProductID, ct.SearchProductAPIKeysJSONRequestBody{
					SortBy:        &sortBy,
					SortDirection: &sortDirection,
				},
			)
			require.NoError(t, sortErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)

			if len(searchResp.JSON200.Items) > 1 {
				for i := 1; i < len(searchResp.JSON200.Items); i++ {
					assert.LessOrEqual(
						t, searchResp.JSON200.Items[i-1].Name, searchResp.JSON200.Items[i].Name,
					)
				}
			}
		},
	)

	t.Run(
		"Search API keys with full text search", func(t *testing.T) {
			fullTextSearch := "Search"
			searchResp, fullTextErr := product.OwnerAuthenticatedClient().SearchProductAPIKeysWithResponse(
				ctx, product.ProductID, ct.SearchProductAPIKeysJSONRequestBody{
					FullTextSearch: &fullTextSearch,
				},
			)
			require.NoError(t, fullTextErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)

			found := false
			for _, apiKey := range searchResp.JSON200.Items {
				if apiKey.Name == apiKey3Name {
					found = true
					break
				}
			}
			assert.True(t, found)
		},
	)

	t.Run(
		"Search API keys with no results", func(t *testing.T) {
			searchResp, noResultsErr := product.OwnerAuthenticatedClient().SearchProductAPIKeysWithResponse(
				ctx, product.ProductID, ct.SearchProductAPIKeysJSONRequestBody{
					Filter: &ct.ProductAPIKeyFilter{
						Names: []string{"NonexistentApiKey"},
					},
				},
			)
			require.NoError(t, noResultsErr)
			assert.Equal(t, 200, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.Empty(t, searchResp.JSON200.Items)
			assert.Equal(t, int64(0), searchResp.JSON200.Total)
		},
	)
}

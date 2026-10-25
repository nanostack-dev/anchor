package itdsl

import (
	"context"
	"net/http"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// ResourceTypeClient drives the product's resource type routes. Every act has
// a *Raw twin returning the untouched response, for the tests whose subject is
// a refusal.
type ResourceTypeClient struct {
	t         *testing.T
	client    *nanostackClient.ClientWithResponses
	productID string
}

// ResourceTypes returns the handle, driven by an API key holding exactly the
// scopes named. Naming none mints an all-scope key.
func (tp *ProductContext) ResourceTypes(scopes ...string) ResourceTypeClient {
	tp.testingContext.Helper()

	client := tp.AllScopeAPIKeyClient()
	if len(scopes) > 0 {
		client, _ = tp.CreateAPIKeyClientWithScopes(scopes)
	}
	return ResourceTypeClient{t: tp.testingContext, client: client, productID: tp.ProductID}
}

func (c ResourceTypeClient) CreateRaw(
	body nanostackClient.CreateProductResourceTypeJSONRequestBody,
) *nanostackClient.CreateProductResourceTypeResponse {
	c.t.Helper()
	resp, err := c.client.CreateProductResourceTypeWithResponse(context.Background(), c.productID, body)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceTypeClient) Create(
	body nanostackClient.CreateProductResourceTypeJSONRequestBody,
) nanostackClient.ProductResourceTypeResponse {
	c.t.Helper()
	resp := c.CreateRaw(body)
	require.Equal(c.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON201)
	return *resp.JSON201
}

// CreateUnder adds a type named after its slug under the given parent types.
func (c ResourceTypeClient) CreateUnder(
	slug string,
	parentTypes ...string,
) nanostackClient.ProductResourceTypeResponse {
	c.t.Helper()
	return c.Create(nanostackClient.CreateProductResourceTypeJSONRequestBody{
		Slug: slug, Name: slug, ParentTypes: parentTypes,
	})
}

func (c ResourceTypeClient) GetRaw(id string) *nanostackClient.GetProductResourceTypeResponse {
	c.t.Helper()
	resp, err := c.client.GetProductResourceTypeWithResponse(context.Background(), c.productID, id)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceTypeClient) Get(id string) nanostackClient.ProductResourceTypeResponse {
	c.t.Helper()
	resp := c.GetRaw(id)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c ResourceTypeClient) UpdateRaw(
	id string, body nanostackClient.UpdateProductResourceTypeJSONRequestBody,
) *nanostackClient.UpdateProductResourceTypeResponse {
	c.t.Helper()
	resp, err := c.client.UpdateProductResourceTypeWithResponse(context.Background(), c.productID, id, body)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceTypeClient) Update(
	id string, body nanostackClient.UpdateProductResourceTypeJSONRequestBody,
) nanostackClient.ProductResourceTypeResponse {
	c.t.Helper()
	resp := c.UpdateRaw(id, body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c ResourceTypeClient) DeleteRaw(id string) *nanostackClient.DeleteProductResourceTypeResponse {
	c.t.Helper()
	resp, err := c.client.DeleteProductResourceTypeWithResponse(context.Background(), c.productID, id)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceTypeClient) Delete(id string) {
	c.t.Helper()
	resp := c.DeleteRaw(id)
	require.Equal(c.t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
}

func (c ResourceTypeClient) SearchRaw(
	body nanostackClient.SearchProductResourceTypesJSONRequestBody,
) *nanostackClient.SearchProductResourceTypesResponse {
	c.t.Helper()
	resp, err := c.client.SearchProductResourceTypesWithResponse(context.Background(), c.productID, body)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceTypeClient) Search(
	body nanostackClient.SearchProductResourceTypesJSONRequestBody,
) nanostackClient.ProductResourceTypeListResponse {
	c.t.Helper()
	resp := c.SearchRaw(body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

// BySlug reads one type through the search's slug filter.
func (c ResourceTypeClient) BySlug(slug string) nanostackClient.ProductResourceTypeResponse {
	c.t.Helper()
	found := c.Search(nanostackClient.SearchProductResourceTypesJSONRequestBody{
		Filter: &nanostackClient.ProductResourceTypeFilter{Slugs: []string{slug}},
	})
	require.Len(c.t, found.Items, 1, "resource type %s", slug)
	return found.Items[0]
}

func (c ResourceTypeClient) ReplaceTreeRaw(
	types ...nanostackClient.ProductResourceTypeTreeItem,
) *nanostackClient.ReplaceProductResourceTypesResponse {
	c.t.Helper()
	resp, err := c.client.ReplaceProductResourceTypesWithResponse(
		context.Background(), c.productID,
		nanostackClient.ReplaceProductResourceTypesJSONRequestBody{ResourceTypes: append(
			[]nanostackClient.ProductResourceTypeTreeItem{}, types...)},
	)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceTypeClient) ReplaceTree(
	types ...nanostackClient.ProductResourceTypeTreeItem,
) []nanostackClient.ProductResourceTypeResponse {
	c.t.Helper()
	resp := c.ReplaceTreeRaw(types...)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return resp.JSON200.ResourceTypes
}

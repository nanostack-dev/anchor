package itdsl

import (
	"context"
	"net/http"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// ResourceClient drives the resource registry of one Organization. Every act
// has a *Raw twin returning the untouched response, for the tests whose
// subject is a refusal.
type ResourceClient struct {
	t              *testing.T
	client         *nanostackClient.ClientWithResponses
	productID      string
	organizationID string
}

// Resources returns the handle for an Organization, driven by an API key
// holding exactly the scopes named. Naming none mints an all-scope key.
func (tp *ProductContext) Resources(organizationID string, scopes ...string) ResourceClient {
	tp.testingContext.Helper()

	client := tp.AllScopeAPIKeyClient()
	if len(scopes) > 0 {
		client, _ = tp.CreateAPIKeyClientWithScopes(scopes)
	}
	return ResourceClient{
		t: tp.testingContext, client: client, productID: tp.ProductID, organizationID: organizationID,
	}
}

// As drives the same Organization with another client, such as another
// product's key or another tenant's platform session.
func (c ResourceClient) As(client *nanostackClient.ClientWithResponses) ResourceClient {
	c.client = client
	return c
}

// Ref names a resource or a parent the way the product does.
func Ref(resourceType, id string) nanostackClient.ResourceParentRef {
	return nanostackClient.ResourceParentRef{Type: resourceType, Id: id}
}

// Top is the Organization as a parent.
func (c ResourceClient) Top() nanostackClient.ResourceParentRef {
	return Ref("organization", c.organizationID)
}

func (c ResourceClient) PutRaw(
	resourceType, id, name string, parent nanostackClient.ResourceParentRef,
) *nanostackClient.PutResourceResponse {
	c.t.Helper()
	resp, err := c.client.PutResourceWithResponse(
		context.Background(), c.productID, c.organizationID, resourceType, id,
		nanostackClient.PutResourceJSONRequestBody{Name: name, Parent: parent},
	)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceClient) Put(
	resourceType, id, name string, parent nanostackClient.ResourceParentRef,
) nanostackClient.ResourceResponse {
	c.t.Helper()
	resp := c.PutRaw(resourceType, id, name, parent)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c ResourceClient) GetRaw(resourceType, id string) *nanostackClient.GetResourceResponse {
	c.t.Helper()
	resp, err := c.client.GetResourceWithResponse(
		context.Background(), c.productID, c.organizationID, resourceType, id,
	)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceClient) Get(resourceType, id string) nanostackClient.ResourceResponse {
	c.t.Helper()
	resp := c.GetRaw(resourceType, id)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c ResourceClient) DeleteRaw(resourceType, id string, cascade bool) *nanostackClient.DeleteResourceResponse {
	c.t.Helper()
	var params *nanostackClient.DeleteResourceParams
	if cascade {
		params = &nanostackClient.DeleteResourceParams{Cascade: &cascade}
	}
	resp, err := c.client.DeleteResourceWithResponse(
		context.Background(), c.productID, c.organizationID, resourceType, id, params,
	)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceClient) Delete(resourceType, id string, cascade bool) {
	c.t.Helper()
	resp := c.DeleteRaw(resourceType, id, cascade)
	require.Equal(c.t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
}

func (c ResourceClient) SearchRaw(
	body nanostackClient.SearchResourcesJSONRequestBody,
) *nanostackClient.SearchResourcesResponse {
	c.t.Helper()
	resp, err := c.client.SearchResourcesWithResponse(context.Background(), c.productID, c.organizationID, body)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceClient) Search(
	body nanostackClient.SearchResourcesJSONRequestBody,
) nanostackClient.ResourceListResponse {
	c.t.Helper()
	resp := c.SearchRaw(body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

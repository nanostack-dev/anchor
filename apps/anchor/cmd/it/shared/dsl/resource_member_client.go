package itdsl

import (
	"context"
	"net/http"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// ResourceMemberClient drives the members of one registered resource. Every
// act has a *Raw twin returning the untouched response, for the tests whose
// subject is a refusal.
type ResourceMemberClient struct {
	t              *testing.T
	client         *nanostackClient.ClientWithResponses
	productID      string
	organizationID string
	resourceType   string
	resourceID     string
}

// ResourceMembers returns the handle for one resource, driven by an API key
// holding exactly the scopes named. Naming none mints an all-scope key.
func (tp *ProductContext) ResourceMembers(
	organizationID, resourceType, resourceID string, scopes ...string,
) ResourceMemberClient {
	tp.testingContext.Helper()

	client := tp.AllScopeAPIKeyClient()
	if len(scopes) > 0 {
		client, _ = tp.CreateAPIKeyClientWithScopes(scopes)
	}
	return ResourceMemberClient{
		t: tp.testingContext, client: client, productID: tp.ProductID, organizationID: organizationID,
		resourceType: resourceType, resourceID: resourceID,
	}
}

// As drives the same resource with another client, such as the owner's
// platform session.
func (c ResourceMemberClient) As(client *nanostackClient.ClientWithResponses) ResourceMemberClient {
	c.client = client
	return c
}

func (c ResourceMemberClient) PutRaw(
	productUserID string,
	roleIDs ...string,
) *nanostackClient.PutResourceMemberResponse {
	c.t.Helper()
	resp, err := c.client.PutResourceMemberWithResponse(
		context.Background(), c.productID, c.organizationID, c.resourceType, c.resourceID, productUserID,
		nanostackClient.PutResourceMemberJSONRequestBody{RoleIds: roleIDs},
	)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceMemberClient) Put(productUserID string, roleIDs ...string) nanostackClient.ResourceMemberResponse {
	c.t.Helper()
	resp := c.PutRaw(productUserID, roleIDs...)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c ResourceMemberClient) GetRaw(productUserID string) *nanostackClient.GetResourceMemberResponse {
	c.t.Helper()
	resp, err := c.client.GetResourceMemberWithResponse(
		context.Background(), c.productID, c.organizationID, c.resourceType, c.resourceID, productUserID,
	)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceMemberClient) Get(productUserID string) nanostackClient.ResourceMemberResponse {
	c.t.Helper()
	resp := c.GetRaw(productUserID)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c ResourceMemberClient) DeleteRaw(productUserID string) *nanostackClient.DeleteResourceMemberResponse {
	c.t.Helper()
	resp, err := c.client.DeleteResourceMemberWithResponse(
		context.Background(), c.productID, c.organizationID, c.resourceType, c.resourceID, productUserID,
	)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceMemberClient) Delete(productUserID string) {
	c.t.Helper()
	resp := c.DeleteRaw(productUserID)
	require.Equal(c.t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
}

func (c ResourceMemberClient) SearchRaw(
	body nanostackClient.SearchResourceMembersJSONRequestBody,
) *nanostackClient.SearchResourceMembersResponse {
	c.t.Helper()
	resp, err := c.client.SearchResourceMembersWithResponse(
		context.Background(), c.productID, c.organizationID, c.resourceType, c.resourceID, body,
	)
	require.NoError(c.t, err)
	return resp
}

func (c ResourceMemberClient) Search(
	body nanostackClient.SearchResourceMembersJSONRequestBody,
) nanostackClient.ResourceMemberListResponse {
	c.t.Helper()
	resp := c.SearchRaw(body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

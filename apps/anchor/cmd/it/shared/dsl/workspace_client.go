package itdsl

import (
	"context"
	"net/http"
	"testing"

	client "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

type WorkspaceClient struct {
	t              *testing.T
	client         *client.ClientWithResponses
	productID      string
	organizationID string
}

func (p *ProductContext) Workspaces(organizationID string) WorkspaceClient {
	p.testingContext.Helper()
	return WorkspaceClient{
		t:              p.testingContext,
		client:         p.AllScopeAPIKeyClient(),
		productID:      p.ProductID,
		organizationID: organizationID,
	}
}

func (c WorkspaceClient) Search(
	body client.SearchOrganizationWorkspacesJSONRequestBody,
) client.ProductWorkspaceListResponse {
	c.t.Helper()
	resp := c.SearchRaw(body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c WorkspaceClient) SearchRaw(
	body client.SearchOrganizationWorkspacesJSONRequestBody,
) *client.SearchOrganizationWorkspacesResponse {
	c.t.Helper()
	resp, err := c.client.SearchOrganizationWorkspacesWithResponse(
		context.Background(),
		c.productID,
		c.organizationID,
		body,
	)
	require.NoError(c.t, err)
	return resp
}

func (c WorkspaceClient) Create(
	body client.CreateOrganizationWorkspaceJSONRequestBody,
) client.ProductWorkspaceResponse {
	c.t.Helper()
	resp := c.CreateRaw(body)
	require.Equal(c.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON201)
	return *resp.JSON201
}

func (c WorkspaceClient) CreateRaw(
	body client.CreateOrganizationWorkspaceJSONRequestBody,
) *client.CreateOrganizationWorkspaceResponse {
	c.t.Helper()
	resp, err := c.client.CreateOrganizationWorkspaceWithResponse(
		context.Background(),
		c.productID,
		c.organizationID,
		body,
	)
	require.NoError(c.t, err)
	return resp
}

func (c WorkspaceClient) Update(
	id string,
	body client.UpdateOrganizationWorkspaceJSONRequestBody,
) client.ProductWorkspaceResponse {
	c.t.Helper()
	resp := c.UpdateRaw(id, body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c WorkspaceClient) UpdateRaw(
	id string,
	body client.UpdateOrganizationWorkspaceJSONRequestBody,
) *client.UpdateOrganizationWorkspaceResponse {
	c.t.Helper()
	resp, err := c.client.UpdateOrganizationWorkspaceWithResponse(
		context.Background(),
		c.productID,
		c.organizationID,
		id,
		body,
	)
	require.NoError(c.t, err)
	return resp
}

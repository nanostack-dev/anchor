package itdsl

import (
	"context"
	"net/http"
	"testing"

	client "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// WorkspaceMemberClient drives the workspace member routes of one organization
// with an all-scope key. Each act carries the require.NoError + status + NotNil
// triplet once, and has a *Raw twin returning the untouched response, for the
// tests whose subject is a refusal.
type WorkspaceMemberClient struct {
	t              *testing.T
	client         *client.ClientWithResponses
	productID      string
	organizationID string
}

func (p *ProductContext) WorkspaceMembers(organizationID string) WorkspaceMemberClient {
	p.testingContext.Helper()
	return WorkspaceMemberClient{
		t:              p.testingContext,
		client:         p.AllScopeAPIKeyClient(),
		productID:      p.ProductID,
		organizationID: organizationID,
	}
}

func (c WorkspaceMemberClient) AssignRaw(
	workspaceID, productUserID, roleID string,
) *client.AssignWorkspaceMemberRoleResponse {
	c.t.Helper()
	resp, err := c.client.AssignWorkspaceMemberRoleWithResponse(
		context.Background(), c.productID, c.organizationID, workspaceID, productUserID,
		client.WorkspaceMemberRoleRequest{RoleId: roleID},
	)
	require.NoError(c.t, err)
	return resp
}

func (c WorkspaceMemberClient) Assign(workspaceID, productUserID, roleID string) client.WorkspaceMemberResponse {
	c.t.Helper()
	resp := c.AssignRaw(workspaceID, productUserID, roleID)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c WorkspaceMemberClient) GetRaw(workspaceID, productUserID string) *client.GetWorkspaceMemberResponse {
	c.t.Helper()
	resp, err := c.client.GetWorkspaceMemberWithResponse(
		context.Background(), c.productID, c.organizationID, workspaceID, productUserID,
	)
	require.NoError(c.t, err)
	return resp
}

func (c WorkspaceMemberClient) Get(workspaceID, productUserID string) client.WorkspaceMemberResponse {
	c.t.Helper()
	resp := c.GetRaw(workspaceID, productUserID)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c WorkspaceMemberClient) RemoveRaw(workspaceID, productUserID string) *client.RemoveWorkspaceMemberResponse {
	c.t.Helper()
	resp, err := c.client.RemoveWorkspaceMemberWithResponse(
		context.Background(), c.productID, c.organizationID, workspaceID, productUserID,
	)
	require.NoError(c.t, err)
	return resp
}

func (c WorkspaceMemberClient) Remove(workspaceID, productUserID string) {
	c.t.Helper()
	resp := c.RemoveRaw(workspaceID, productUserID)
	require.Equal(c.t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
}

func (c WorkspaceMemberClient) SearchRaw(
	workspaceID string, body client.SearchWorkspaceMembersJSONRequestBody,
) *client.SearchWorkspaceMembersResponse {
	c.t.Helper()
	resp, err := c.client.SearchWorkspaceMembersWithResponse(
		context.Background(), c.productID, c.organizationID, workspaceID, body,
	)
	require.NoError(c.t, err)
	return resp
}

func (c WorkspaceMemberClient) Search(
	workspaceID string, body client.SearchWorkspaceMembersJSONRequestBody,
) client.WorkspaceMemberListResponse {
	c.t.Helper()
	resp := c.SearchRaw(workspaceID, body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

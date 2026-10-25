package itdsl

import (
	"context"
	"net/http"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// OrganizationRoleClient drives one Organization's role routes. Every act has a
// *Raw twin returning the untouched response, for the tests whose subject is a
// refusal.
type OrganizationRoleClient struct {
	t              *testing.T
	client         *nanostackClient.ClientWithResponses
	productID      string
	organizationID string
}

// OrganizationRoles returns the handle for one Organization, driven by an
// all-scope API key.
func (tp *ProductContext) OrganizationRoles(organizationID string) OrganizationRoleClient {
	tp.testingContext.Helper()
	return OrganizationRoleClient{
		t:              tp.testingContext,
		client:         tp.AllScopeAPIKeyClient(),
		productID:      tp.ProductID,
		organizationID: organizationID,
	}
}

func (c OrganizationRoleClient) SearchRaw(
	body nanostackClient.SearchOrganizationRolesJSONRequestBody,
) *nanostackClient.SearchOrganizationRolesResponse {
	c.t.Helper()
	resp, err := c.client.SearchOrganizationRolesWithResponse(context.Background(), c.productID, c.organizationID, body)
	require.NoError(c.t, err)
	return resp
}

func (c OrganizationRoleClient) Search(
	body nanostackClient.SearchOrganizationRolesJSONRequestBody,
) nanostackClient.OrganizationRoleListResponse {
	c.t.Helper()
	resp := c.SearchRaw(body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c OrganizationRoleClient) GetRaw(roleID string) *nanostackClient.GetOrganizationRoleResponse {
	c.t.Helper()
	resp, err := c.client.GetOrganizationRoleWithResponse(context.Background(), c.productID, c.organizationID, roleID)
	require.NoError(c.t, err)
	return resp
}

func (c OrganizationRoleClient) Get(roleID string) nanostackClient.OrganizationRoleResponse {
	c.t.Helper()
	resp := c.GetRaw(roleID)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c OrganizationRoleClient) UpdateRaw(
	roleID string, body nanostackClient.UpdateOrganizationRoleJSONRequestBody,
) *nanostackClient.UpdateOrganizationRoleResponse {
	c.t.Helper()
	resp, err := c.client.UpdateOrganizationRoleWithResponse(
		context.Background(), c.productID, c.organizationID, roleID, body,
	)
	require.NoError(c.t, err)
	return resp
}

func (c OrganizationRoleClient) Update(
	roleID string, body nanostackClient.UpdateOrganizationRoleJSONRequestBody,
) nanostackClient.OrganizationRoleResponse {
	c.t.Helper()
	resp := c.UpdateRaw(roleID, body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c OrganizationRoleClient) CreateRaw(
	body nanostackClient.CreateOrganizationRoleJSONRequestBody,
) *nanostackClient.CreateOrganizationRoleResponse {
	c.t.Helper()
	resp, err := c.client.CreateOrganizationRoleWithResponse(context.Background(), c.productID, c.organizationID, body)
	require.NoError(c.t, err)
	return resp
}

func (c OrganizationRoleClient) Create(
	body nanostackClient.CreateOrganizationRoleJSONRequestBody,
) nanostackClient.OrganizationRoleResponse {
	c.t.Helper()
	resp := c.CreateRaw(body)
	require.Equal(c.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON201)
	return *resp.JSON201
}

func (c OrganizationRoleClient) DeleteRaw(roleID string) *nanostackClient.DeleteOrganizationRoleResponse {
	c.t.Helper()
	resp, err := c.client.DeleteOrganizationRoleWithResponse(
		context.Background(),
		c.productID,
		c.organizationID,
		roleID,
	)
	require.NoError(c.t, err)
	return resp
}

func (c OrganizationRoleClient) ResetRaw(roleID string) *nanostackClient.ResetOrganizationRoleResponse {
	c.t.Helper()
	resp, err := c.client.ResetOrganizationRoleWithResponse(context.Background(), c.productID, c.organizationID, roleID)
	require.NoError(c.t, err)
	return resp
}

func (c OrganizationRoleClient) Reset(roleID string) nanostackClient.OrganizationRoleResponse {
	c.t.Helper()
	resp := c.ResetRaw(roleID)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

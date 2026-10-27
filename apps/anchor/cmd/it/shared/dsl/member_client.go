package itdsl

import (
	"context"
	"net/http"
	"strings"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// MemberClient drives one organization's member routes with one credential.
// Each act carries the require.NoError + status + NotNil triplet once; the
// *Raw twins return the untouched response for the tests whose subject is a
// refusal.
type MemberClient struct {
	t              *testing.T
	client         *nanostackClient.ClientWithResponses
	productID      string
	organizationID string
}

// Members returns the handle for the organization, driven by an all-scope key.
func (tp *ProductContext) Members(organizationID string) MemberClient {
	tp.testingContext.Helper()
	return MemberClient{
		t: tp.testingContext, client: tp.AllScopeAPIKeyClient(),
		productID: tp.ProductID, organizationID: organizationID,
	}
}

// Add adds the product user with the role, or with no role when roleID is nil.
func (c MemberClient) Add(productUserID string, roleID *string) nanostackClient.OrganizationMemberResponse {
	c.t.Helper()
	resp, err := c.client.AddOrganizationMemberWithResponse(
		context.Background(), c.productID, c.organizationID,
		nanostackClient.AddOrganizationMemberJSONRequestBody{ProductUserId: productUserID, RoleId: roleID},
	)
	require.NoError(c.t, err)
	require.Equal(c.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON201)
	return *resp.JSON201
}

// SetRole replaces the member's role; a nil roleID sends null, which removes it.
func (c MemberClient) SetRole(productUserID string, roleID *string) nanostackClient.OrganizationMemberResponse {
	c.t.Helper()
	resp, err := c.client.UpdateOrganizationMemberRoleWithResponse(
		context.Background(), c.productID, c.organizationID, productUserID,
		nanostackClient.UpdateOrganizationMemberRoleJSONRequestBody{RoleId: roleID},
	)
	require.NoError(c.t, err)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

// SetRoleRawBody sends the JSON as written, for the tests whose subject is a
// body the typed request cannot build.
func (c MemberClient) SetRoleRawBody(
	productUserID, body string,
) *nanostackClient.UpdateOrganizationMemberRoleResponse {
	c.t.Helper()
	resp, err := c.client.UpdateOrganizationMemberRoleWithBodyWithResponse(
		context.Background(), c.productID, c.organizationID, productUserID,
		"application/json", strings.NewReader(body),
	)
	require.NoError(c.t, err)
	return resp
}

// Get reads the member, with the role's permissions.
func (c MemberClient) Get(productUserID string) nanostackClient.OrganizationMemberResponse {
	c.t.Helper()
	include := []nanostackClient.OrganizationMemberInclude{nanostackClient.OrganizationMemberIncludeRolePermissions}
	resp, err := c.client.GetOrganizationMemberWithResponse(
		context.Background(), c.productID, c.organizationID, productUserID,
		&nanostackClient.GetOrganizationMemberParams{Include: &include},
	)
	require.NoError(c.t, err)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

// Organization reads the membership from the product user's side.
func (c MemberClient) Organization(productUserID string) nanostackClient.UserOrganizationResponse {
	c.t.Helper()
	resp, err := c.client.GetUserOrganizationWithResponse(
		context.Background(), c.productID, productUserID, c.organizationID,
		&nanostackClient.GetUserOrganizationParams{},
	)
	require.NoError(c.t, err)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

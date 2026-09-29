package itdsl

import (
	"context"
	"net/http"
	"strings"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// InvitationClient drives the organization invitation routes with one
// credential. Each act carries the require.NoError + status + NotNil triplet
// once, so a test body states what it does and what it expects, and nothing
// else. Every act has a *Raw twin returning the untouched response, for the
// tests whose subject is a refusal.
type InvitationClient struct {
	t         *testing.T
	client    *nanostackClient.ClientWithResponses
	productID string
}

// Invitations returns the handle, driven by an API key holding exactly the
// scopes named. Naming none mints an all-scope key.
func (tp *ProductContext) Invitations(scopes ...string) InvitationClient {
	tp.testingContext.Helper()

	client := tp.AllScopeAPIKeyClient()
	if len(scopes) > 0 {
		client, _ = tp.CreateAPIKeyClientWithScopes(scopes)
	}

	return InvitationClient{t: tp.testingContext, client: client, productID: tp.ProductID}
}

// As swaps the credential, for the tests whose subject is a scope.
func (c InvitationClient) As(client *nanostackClient.ClientWithResponses) InvitationClient {
	c.client = client
	return c
}

func (c InvitationClient) CreateRaw(
	organizationID string, body nanostackClient.CreateOrganizationInvitationJSONRequestBody,
) *nanostackClient.CreateOrganizationInvitationResponse {
	c.t.Helper()
	resp, err := c.client.CreateOrganizationInvitationWithResponse(
		context.Background(), c.productID, organizationID, body,
	)
	require.NoError(c.t, err)
	return resp
}

// CreateRawBody sends the JSON as written, for the tests whose subject is a
// body the typed client refuses to build.
func (c InvitationClient) CreateRawBody(
	organizationID, body string,
) *nanostackClient.CreateOrganizationInvitationResponse {
	c.t.Helper()
	resp, err := c.client.CreateOrganizationInvitationWithBodyWithResponse(
		context.Background(), c.productID, organizationID, "application/json", strings.NewReader(body),
	)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationClient) Create(
	organizationID string, body nanostackClient.CreateOrganizationInvitationJSONRequestBody,
) nanostackClient.CreatedOrganizationInvitationResponse {
	c.t.Helper()
	resp := c.CreateRaw(organizationID, body)
	require.Equal(c.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON201)
	return *resp.JSON201
}

func (c InvitationClient) GetRaw(
	organizationID, invitationID string,
) *nanostackClient.GetOrganizationInvitationResponse {
	c.t.Helper()
	resp, err := c.client.GetOrganizationInvitationWithResponse(
		context.Background(), c.productID, organizationID, invitationID,
	)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationClient) Get(organizationID, invitationID string) nanostackClient.OrganizationInvitationResponse {
	c.t.Helper()
	resp := c.GetRaw(organizationID, invitationID)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c InvitationClient) SearchRaw(
	organizationID string, body nanostackClient.SearchOrganizationInvitationsJSONRequestBody,
) *nanostackClient.SearchOrganizationInvitationsResponse {
	c.t.Helper()
	resp, err := c.client.SearchOrganizationInvitationsWithResponse(
		context.Background(), c.productID, organizationID, body,
	)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationClient) Search(
	organizationID string, body nanostackClient.SearchOrganizationInvitationsJSONRequestBody,
) []nanostackClient.OrganizationInvitationResponse {
	c.t.Helper()
	resp := c.SearchRaw(organizationID, body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return resp.JSON200.Items
}

// SearchByStatus lists the organization's invitations holding one of the statuses.
func (c InvitationClient) SearchByStatus(
	organizationID string, statuses ...nanostackClient.OrganizationInvitationStatus,
) []nanostackClient.OrganizationInvitationResponse {
	c.t.Helper()
	return c.Search(organizationID, nanostackClient.SearchOrganizationInvitationsJSONRequestBody{
		Filter: &nanostackClient.OrganizationInvitationFilter{Statuses: &statuses},
	})
}

// UpdateRawBody sends the JSON as written, for the tests whose subject is a
// field the typed request has no room for.
func (c InvitationClient) UpdateRawBody(
	organizationID, invitationID, body string,
) *nanostackClient.UpdateOrganizationInvitationResponse {
	c.t.Helper()
	resp, err := c.client.UpdateOrganizationInvitationWithBodyWithResponse(
		context.Background(), c.productID, organizationID, invitationID, "application/json", strings.NewReader(body),
	)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationClient) UpdateRaw(
	organizationID, invitationID string, body nanostackClient.UpdateOrganizationInvitationJSONRequestBody,
) *nanostackClient.UpdateOrganizationInvitationResponse {
	c.t.Helper()
	resp, err := c.client.UpdateOrganizationInvitationWithResponse(
		context.Background(), c.productID, organizationID, invitationID, body,
	)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationClient) Update(
	organizationID, invitationID string, body nanostackClient.UpdateOrganizationInvitationJSONRequestBody,
) nanostackClient.OrganizationInvitationResponse {
	c.t.Helper()
	resp := c.UpdateRaw(organizationID, invitationID, body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c InvitationClient) DeleteRaw(
	organizationID, invitationID string,
) *nanostackClient.DeleteOrganizationInvitationResponse {
	c.t.Helper()
	resp, err := c.client.DeleteOrganizationInvitationWithResponse(
		context.Background(), c.productID, organizationID, invitationID,
	)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationClient) Delete(organizationID, invitationID string) {
	c.t.Helper()
	resp := c.DeleteRaw(organizationID, invitationID)
	require.Equal(c.t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
}

func (c InvitationClient) ResendRaw(
	organizationID, invitationID string,
) *nanostackClient.ResendOrganizationInvitationResponse {
	c.t.Helper()
	resp, err := c.client.ResendOrganizationInvitationWithResponse(
		context.Background(), c.productID, organizationID, invitationID,
	)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationClient) Resend(
	organizationID, invitationID string,
) nanostackClient.CreatedOrganizationInvitationResponse {
	c.t.Helper()
	resp := c.ResendRaw(organizationID, invitationID)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c InvitationClient) LookupRaw(token string) *nanostackClient.LookupOrganizationInvitationResponse {
	c.t.Helper()
	resp, err := c.client.LookupOrganizationInvitationWithResponse(
		context.Background(), c.productID,
		nanostackClient.LookupOrganizationInvitationJSONRequestBody{Token: token},
	)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationClient) Lookup(token string) nanostackClient.OrganizationInvitationResponse {
	c.t.Helper()
	resp := c.LookupRaw(token)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c InvitationClient) AcceptRaw(
	token, productUserID string,
) *nanostackClient.AcceptOrganizationInvitationResponse {
	c.t.Helper()
	resp, err := c.client.AcceptOrganizationInvitationWithResponse(
		context.Background(), c.productID,
		nanostackClient.AcceptOrganizationInvitationJSONRequestBody{Token: token, ProductUserId: productUserID},
	)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationClient) Accept(token, productUserID string) nanostackClient.OrganizationInvitationResponse {
	c.t.Helper()
	resp := c.AcceptRaw(token, productUserID)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

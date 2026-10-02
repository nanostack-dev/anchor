package itdsl

import (
	"context"
	"net/http"
	"strings"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
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
) nanostackClient.OrganizationInvitationResponse {
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

// SearchByEmail lists the organization's invitations addressed to one of the
// email addresses.
func (c InvitationClient) SearchByEmail(
	organizationID string, emails ...string,
) []nanostackClient.OrganizationInvitationResponse {
	c.t.Helper()
	return c.Search(organizationID, nanostackClient.SearchOrganizationInvitationsJSONRequestBody{
		Filter: &nanostackClient.OrganizationInvitationFilter{Emails: typedEmails(emails)},
	})
}

func (c InvitationClient) SearchInProductRaw(
	body nanostackClient.SearchProductOrganizationInvitationsJSONRequestBody,
) *nanostackClient.SearchProductOrganizationInvitationsResponse {
	c.t.Helper()
	resp, err := c.client.SearchProductOrganizationInvitationsWithResponse(context.Background(), c.productID, body)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationClient) SearchInProduct(
	body nanostackClient.SearchProductOrganizationInvitationsJSONRequestBody,
) []nanostackClient.OrganizationInvitationResponse {
	c.t.Helper()
	resp := c.SearchInProductRaw(body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return resp.JSON200.Items
}

// PendingFor lists the pending invitations of every organization of the
// product addressed to one of the email addresses: the call a Product makes
// when a person signs in.
func (c InvitationClient) PendingFor(emails ...string) []nanostackClient.OrganizationInvitationResponse {
	c.t.Helper()
	statuses := []nanostackClient.OrganizationInvitationStatus{nanostackClient.Pending}
	return c.SearchInProduct(nanostackClient.SearchProductOrganizationInvitationsJSONRequestBody{
		Filter: &nanostackClient.OrganizationInvitationFilter{Statuses: &statuses, Emails: typedEmails(emails)},
	})
}

func typedEmails(emails []string) *[]openapi_types.Email {
	typed := make([]openapi_types.Email, 0, len(emails))
	for _, email := range emails {
		typed = append(typed, openapi_types.Email(email))
	}
	return &typed
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

func (c InvitationClient) AcceptRaw(
	organizationID, invitationID, productUserID string,
) *nanostackClient.AcceptOrganizationInvitationResponse {
	c.t.Helper()
	resp, err := c.client.AcceptOrganizationInvitationWithResponse(
		context.Background(), c.productID, organizationID, invitationID,
		nanostackClient.AcceptOrganizationInvitationJSONRequestBody{ProductUserId: productUserID},
	)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationClient) Accept(
	organizationID, invitationID, productUserID string,
) nanostackClient.OrganizationInvitationResponse {
	c.t.Helper()
	resp := c.AcceptRaw(organizationID, invitationID, productUserID)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

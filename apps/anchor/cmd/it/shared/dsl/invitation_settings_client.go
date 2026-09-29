package itdsl

import (
	"context"
	"net/http"
	"strings"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// InvitationSettingsClient drives the invitation settings routes with one
// credential. It starts as the Platform User who owns the Product, the only
// principal the routes accept. Every act has a *Raw twin returning the
// untouched response, for the tests whose subject is a refusal.
type InvitationSettingsClient struct {
	t         *testing.T
	client    *nanostackClient.ClientWithResponses
	productID string
}

func (tp *ProductContext) InvitationSettings() InvitationSettingsClient {
	tp.testingContext.Helper()
	return InvitationSettingsClient{
		t:         tp.testingContext,
		client:    tp.OwnerAuthenticatedClient(),
		productID: tp.ProductID,
	}
}

// As swaps the credential, for the tests whose subject is who may call.
func (c InvitationSettingsClient) As(client *nanostackClient.ClientWithResponses) InvitationSettingsClient {
	c.client = client
	return c
}

func (c InvitationSettingsClient) GetRaw() *nanostackClient.GetInvitationSettingsResponse {
	c.t.Helper()
	resp, err := c.client.GetInvitationSettingsWithResponse(context.Background(), c.productID)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationSettingsClient) Get() nanostackClient.InvitationSettingsResponse {
	c.t.Helper()
	resp := c.GetRaw()
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c InvitationSettingsClient) UpdateRaw(
	body nanostackClient.UpdateInvitationSettingsJSONRequestBody,
) *nanostackClient.UpdateInvitationSettingsResponse {
	c.t.Helper()
	resp, err := c.client.UpdateInvitationSettingsWithResponse(context.Background(), c.productID, body)
	require.NoError(c.t, err)
	return resp
}

// UpdateRawBody sends the JSON as written, for the tests whose subject is a
// body the typed request refuses to build.
func (c InvitationSettingsClient) UpdateRawBody(body string) *nanostackClient.UpdateInvitationSettingsResponse {
	c.t.Helper()
	resp, err := c.client.UpdateInvitationSettingsWithBodyWithResponse(
		context.Background(), c.productID, "application/json", strings.NewReader(body),
	)
	require.NoError(c.t, err)
	return resp
}

func (c InvitationSettingsClient) Update(
	body nanostackClient.UpdateInvitationSettingsJSONRequestBody,
) nanostackClient.InvitationSettingsResponse {
	c.t.Helper()
	resp := c.UpdateRaw(body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

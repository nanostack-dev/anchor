package itdsl

import (
	"context"
	"net/http"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/require"
)

// OrganizationClient drives the product's organization routes with one
// credential. Each act carries the require.NoError + status + NotNil triplet
// once, so a test body states what it does and what it expects, and nothing
// else. Every act has a *Raw twin returning the untouched response, for the
// tests whose subject is a refusal.
type OrganizationClient struct {
	t         *testing.T
	client    *nanostackClient.ClientWithResponses
	productID string
}

// Organizations returns the handle, driven by an API key holding exactly the
// scopes named. Naming none mints an all-scope key: a test that is not about
// authorization should not have to list what it needs, and a test that is
// should say so where the reader can see it.
func (tp *ProductContext) Organizations(scopes ...string) OrganizationClient {
	tp.testingContext.Helper()

	client := tp.AllScopeAPIKeyClient()
	if len(scopes) > 0 {
		client, _ = tp.CreateAPIKeyClientWithScopes(scopes)
	}

	return OrganizationClient{t: tp.testingContext, client: client, productID: tp.ProductID}
}

// As swaps the credential, for the tests whose subject is a scope.
func (c OrganizationClient) As(client *nanostackClient.ClientWithResponses) OrganizationClient {
	c.client = client
	return c
}

func (c OrganizationClient) CreateRaw(
	body nanostackClient.CreateProductOrganizationJSONRequestBody,
) *nanostackClient.CreateProductOrganizationResponse {
	c.t.Helper()
	resp, err := c.client.CreateProductOrganizationWithResponse(
		context.Background(), c.productID, body,
	)
	require.NoError(c.t, err)
	return resp
}

func (c OrganizationClient) Create(
	body nanostackClient.CreateProductOrganizationJSONRequestBody,
) nanostackClient.ProductOrganizationResponse {
	c.t.Helper()
	resp := c.CreateRaw(body)
	require.Equal(c.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON201)
	return *resp.JSON201
}

func (c OrganizationClient) GetRaw(
	organizationID string, include ...nanostackClient.OrganizationInclude,
) *nanostackClient.GetProductOrganizationResponse {
	c.t.Helper()
	resp, err := c.client.GetProductOrganizationWithResponse(
		context.Background(), c.productID, organizationID, getIncludeParams(include),
	)
	require.NoError(c.t, err)
	return resp
}

// Get reads one organization, naming the related resources to read with it.
func (c OrganizationClient) Get(
	organizationID string, include ...nanostackClient.OrganizationInclude,
) nanostackClient.ProductOrganizationResponse {
	c.t.Helper()
	resp := c.GetRaw(organizationID, include...)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c OrganizationClient) SearchRaw(
	body nanostackClient.SearchProductOrganizationsJSONRequestBody,
	include ...nanostackClient.OrganizationInclude,
) *nanostackClient.SearchProductOrganizationsResponse {
	c.t.Helper()
	resp, err := c.client.SearchProductOrganizationsWithResponse(
		context.Background(), c.productID, searchIncludeParams(include), body,
	)
	require.NoError(c.t, err)
	return resp
}

func (c OrganizationClient) Search(
	body nanostackClient.SearchProductOrganizationsJSONRequestBody,
	include ...nanostackClient.OrganizationInclude,
) []nanostackClient.ProductOrganizationResponse {
	c.t.Helper()
	resp := c.SearchRaw(body, include...)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return resp.JSON200.Items
}

// SearchNamed finds the organizations of the product carrying that name.
func (c OrganizationClient) SearchNamed(
	name string, include ...nanostackClient.OrganizationInclude,
) []nanostackClient.ProductOrganizationResponse {
	c.t.Helper()
	return c.Search(
		nanostackClient.SearchProductOrganizationsJSONRequestBody{
			Filter: &nanostackClient.OrganizationFilter{Names: []string{name}},
		},
		include...,
	)
}

// CountNamed reports how many organizations of the product carry that name. It
// is how a rolled-back create is told apart from a create that never ran.
func (c OrganizationClient) CountNamed(name string) int {
	c.t.Helper()
	return len(c.SearchNamed(name))
}

// UniqueOrganizationName keeps two tests in the same package from colliding on
// a name they search by, without making the name itself the subject of a test.
func UniqueOrganizationName() string {
	return "org_" + ids.MustNew("ct")
}

func getIncludeParams(
	include []nanostackClient.OrganizationInclude,
) *nanostackClient.GetProductOrganizationParams {
	params := &nanostackClient.GetProductOrganizationParams{}
	if len(include) > 0 {
		params.Include = &include
	}
	return params
}

func searchIncludeParams(
	include []nanostackClient.OrganizationInclude,
) *nanostackClient.SearchProductOrganizationsParams {
	params := &nanostackClient.SearchProductOrganizationsParams{}
	if len(include) > 0 {
		params.Include = &include
	}
	return params
}

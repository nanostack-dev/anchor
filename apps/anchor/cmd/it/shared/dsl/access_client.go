package itdsl

import (
	"context"
	"net/http"
	"testing"

	client "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// AccessClient calls access evaluation with a product API key holding exactly
// the scopes the route requires.
type AccessClient struct {
	t         *testing.T
	client    *client.ClientWithResponses
	productID string
}

func (p *ProductContext) Access() AccessClient {
	p.testingContext.Helper()
	evaluator, _ := p.CreateAPIKeyClientWithScopes([]string{"product_user:read", "organization_api_key:read"})
	return AccessClient{t: p.testingContext, client: evaluator, productID: p.ProductID}
}

// AccessClientWith uses another credential, for tests whose subject is the caller's scopes.
func (p *ProductContext) AccessClientWith(credential *client.ClientWithResponses) AccessClient {
	p.testingContext.Helper()
	return AccessClient{t: p.testingContext, client: credential, productID: p.ProductID}
}

func (c AccessClient) EvaluateBatch(body client.EvaluateAccessBatchJSONRequestBody) client.AccessEvaluationsResponse {
	c.t.Helper()
	resp := c.EvaluateBatchRaw(body)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c AccessClient) EvaluateBatchRaw(
	body client.EvaluateAccessBatchJSONRequestBody,
) *client.EvaluateAccessBatchResponse {
	c.t.Helper()
	resp, err := c.client.EvaluateAccessBatchWithResponse(context.Background(), c.productID, body)
	require.NoError(c.t, err)
	return resp
}

func (c AccessClient) Evaluate(body client.EvaluateAccessJSONRequestBody) client.AccessEvaluationResponse {
	c.t.Helper()
	resp, err := c.client.EvaluateAccessWithResponse(context.Background(), c.productID, body)
	require.NoError(c.t, err)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

func (c AccessClient) EvaluateRaw(body client.EvaluateAccessJSONRequestBody) (*client.EvaluateAccessResponse, error) {
	c.t.Helper()
	return c.client.EvaluateAccessWithResponse(context.Background(), c.productID, body)
}

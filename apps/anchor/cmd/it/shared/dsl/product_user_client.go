package itdsl

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	client "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// ProductUserClient calls the product user routes with an all-scope product API key.
type ProductUserClient struct {
	t         *testing.T
	client    *client.ClientWithResponses
	productID string
}

func (p *ProductContext) ProductUsers() ProductUserClient {
	p.testingContext.Helper()
	return ProductUserClient{t: p.testingContext, client: p.AllScopeAPIKeyClient(), productID: p.ProductID}
}

func (c ProductUserClient) Create(body client.CreateProductUserJSONRequestBody) client.ProductUserResponse {
	c.t.Helper()
	resp := c.CreateRaw(body)
	require.Equal(c.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON201)
	return *resp.JSON201
}

func (c ProductUserClient) CreateRaw(body client.CreateProductUserJSONRequestBody) *client.CreateProductUserResponse {
	c.t.Helper()
	resp, err := c.client.CreateProductUserWithResponse(context.Background(), c.productID, body)
	require.NoError(c.t, err)
	return resp
}

func (c ProductUserClient) Get(productUserID string) client.ProductUserResponse {
	c.t.Helper()
	resp, err := c.client.GetProductUserWithResponse(context.Background(), c.productID, productUserID)
	require.NoError(c.t, err)
	require.Equal(c.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(c.t, resp.JSON200)
	return *resp.JSON200
}

// RawResponse is an untouched status and body.
type RawResponse struct {
	StatusCode int
	Body       []byte
}

// SendRaw sends any method to one product user's path with the same credential,
// for tests about a method the contract does not declare.
func (c ProductUserClient) SendRaw(method, productUserID string, body any) RawResponse {
	c.t.Helper()
	generated, ok := c.client.ClientInterface.(*client.Client)
	require.True(c.t, ok, "the generated client wraps *client.Client")

	target, err := url.JoinPath(generated.Server, "v1", "products", c.productID, "product-users", productUserID)
	require.NoError(c.t, err)
	payload, err := json.Marshal(body)
	require.NoError(c.t, err)
	req, err := http.NewRequestWithContext(context.Background(), method, target, bytes.NewReader(payload))
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	for _, edit := range generated.RequestEditors {
		require.NoError(c.t, edit(req.Context(), req))
	}

	resp, err := generated.Client.Do(req)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	return RawResponse{StatusCode: resp.StatusCode, Body: respBody}
}

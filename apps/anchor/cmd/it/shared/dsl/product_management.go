package itdsl

import (
	"net/http"

	client "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

type ProductManagementClient struct {
	product *ProductContext
	client  *client.ClientWithResponses
}

func (tp *ProductContext) Management() ProductManagementClient {
	return ProductManagementClient{product: tp, client: tp.OwnerAuthenticatedClient()}
}

func (c ProductManagementClient) As(other *client.ClientWithResponses) ProductManagementClient {
	c.client = other
	return c
}

func (c ProductManagementClient) Create(body client.CreateProductJSONRequestBody) client.ProductResponse {
	t := c.product.testingContext
	t.Helper()
	resp, err := c.client.CreateProductWithResponse(t.Context(), body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON201)
	return *resp.JSON201
}

func (c ProductManagementClient) GetRaw() *client.GetProductResponse {
	t := c.product.testingContext
	t.Helper()
	resp, err := c.client.GetProductWithResponse(t.Context(), c.product.ProductID)
	require.NoError(t, err)
	return resp
}

func (c ProductManagementClient) Get() client.ProductResponse {
	t := c.product.testingContext
	t.Helper()
	resp := c.GetRaw()
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON200)
	return *resp.JSON200
}

func (c ProductManagementClient) UpdateRaw(body client.UpdateProductJSONRequestBody) *client.UpdateProductResponse {
	t := c.product.testingContext
	t.Helper()
	resp, err := c.client.UpdateProductWithResponse(t.Context(), c.product.ProductID, body)
	require.NoError(t, err)
	return resp
}

func (c ProductManagementClient) Update(body client.UpdateProductJSONRequestBody) client.ProductResponse {
	t := c.product.testingContext
	t.Helper()
	resp := c.UpdateRaw(body)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON200)
	return *resp.JSON200
}

func (c ProductManagementClient) DeleteRaw() *client.DeleteProductResponse {
	t := c.product.testingContext
	t.Helper()
	resp, err := c.client.DeleteProductWithResponse(t.Context(), c.product.ProductID)
	require.NoError(t, err)
	return resp
}

func (c ProductManagementClient) Delete() {
	t := c.product.testingContext
	t.Helper()
	resp := c.DeleteRaw()
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
}

package ct_test

import (
	"context"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteProductUser(t *testing.T) {
	ctx := context.Background()

	t.Run(
		"SuccessfulDeleteProductUser", func(t *testing.T) {
			productContext := createTestProductContext(t)

			testUser := createDSLProductUser(t, productContext)
			assert.NotEmpty(t, testUser.ID, "test product user should have ID")

			// Use API key client with product_user:delete scope
			apiKeyDeleteClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:delete"})

			resp, err := apiKeyDeleteClient.DeleteProductUserWithResponse(
				ctx, productContext.ProductID, testUser.ID,
			)
			require.NoError(t, err, "delete product productuserservice request should not error")
			assert.Equal(
				t, 204, resp.StatusCode(),
				"delete product productuserservice should return 204 No Content",
			)

			// Use API key client with product_user:read scope to verify deletion
			apiKeyReadClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:read"})

			getResp, err := apiKeyReadClient.GetProductUserWithResponse(
				ctx, productContext.ProductID, testUser.ID,
			)
			require.NoError(
				t, err, "get deleted product productuserservice request should not error",
			)
			assert.Equal(
				t, 404, getResp.StatusCode(),
				"get deleted product productuserservice should return 404 Not Found",
			)
		},
	)

	t.Run(
		"DeleteNonExistentProductUser", func(t *testing.T) {
			productContext := createTestProductContext(t)

			nonExistentUserID := ids.MustNew("puser")

			// Use API key client with product_user:delete scope
			apiKeyDeleteClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:delete"})

			resp, err := apiKeyDeleteClient.DeleteProductUserWithResponse(
				ctx, productContext.ProductID, nonExistentUserID,
			)
			require.NoError(
				t, err, "delete non-existent product productuserservice request should not error",
			)
			assert.Equal(
				t, 204, resp.StatusCode(),
				"delete non-existent product productuserservice should return 204 Not Found",
			)
		},
	)

	t.Run(
		"DeleteProductUserMultipleTimes", func(t *testing.T) {
			productContext := createTestProductContext(t)

			testUser := createDSLProductUser(t, productContext)

			// Use API key client with product_user:delete scope
			apiKeyDeleteClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:delete"})

			resp1, err := apiKeyDeleteClient.DeleteProductUserWithResponse(
				ctx, productContext.ProductID, testUser.ID,
			)
			require.NoError(t, err, "first delete should not error")
			assert.Equal(t, 204, resp1.StatusCode(), "first delete should return 204 No Content")

			resp2, err := apiKeyDeleteClient.DeleteProductUserWithResponse(
				ctx, productContext.ProductID, testUser.ID,
			)
			require.NoError(t, err, "second delete should not error")
			assert.Equal(t, 204, resp2.StatusCode(), "second delete should return 204 Not Found")
		},
	)
}

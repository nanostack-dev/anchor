package ct_test

import (
	"context"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProductUser(t *testing.T) {
	ctx := context.Background()

	t.Run(
		"GetNonExistentProductUser", func(t *testing.T) {
			productContext := createTestProductContext(t)

			nonExistentUserID := ids.MustNew("puser")

			// Use API key client with product_user:read scope
			apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:read"})

			resp, err := apiKeyClient.GetProductUserWithResponse(
				ctx, productContext.ProductID, nonExistentUserID,
			)
			require.NoError(
				t, err, "get non-existent product productuserservice request should not error",
			)
			assert.Equal(
				t, 404, resp.StatusCode(),
				"get non-existent product productuserservice should return 404 Not Found",
			)
		},
	)

	t.Run(
		"GetWithInvalidUserID", func(t *testing.T) {
			// Create test product with API key
			productContext := createTestProductContext(t)

			invalidUserID := "invalid-productuserservice-id"

			// Use API key client with product_user:read scope
			apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:read"})

			resp, err := apiKeyClient.GetProductUserWithResponse(
				ctx, productContext.ProductID, invalidUserID,
			)
			require.NoError(
				t, err, "get with invalid productuserservice ID request should not error",
			)
			assert.GreaterOrEqual(t, resp.StatusCode(), 400, "should return client error")
		},
	)

	t.Run(
		"SuccessfulGetOfExistingProductUser", func(t *testing.T) {
			// Create test product with API key
			productContext := createTestProductContext(t)

			// Create a test product user first via DSL
			testUser := createDSLProductUser(t, productContext)
			assert.NotEmpty(t, testUser.ID, "test product user should have ID")

			// Now get the productuserservice
			// Use API key client with product_user:read scope
			apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:read"})

			resp, err := apiKeyClient.GetProductUserWithResponse(
				ctx, productContext.ProductID, testUser.ID,
			)
			require.NoError(
				t, err, "get created product productuserservice request should not error",
			)
			assert.Equal(
				t, 200, resp.StatusCode(),
				"get created product productuserservice should return 200 OK",
			)
			if assert.NotNil(t, resp.JSON200) {
				assert.Equal(t, testUser.ID, resp.JSON200.Id)
				assert.Equal(t, resp.JSON200.Email, testUser.Email)
				assert.Equal(t, *resp.JSON200.Name, testUser.Name)
				assert.Equal(t, string(resp.JSON200.Status), testUser.Status)
				assert.NotZero(t, resp.JSON200.CreatedAt)
				assert.NotZero(t, resp.JSON200.UpdatedAt)
			}
		},
	)
}

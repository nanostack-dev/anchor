package ct_test

import (
	"context"
	"net/http"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchProductUsers(t *testing.T) {
	ctx := context.Background()

	t.Run(
		"SearchProductUsersEmpty", func(t *testing.T) {
			productContext := createTestProductContext(t)

			resp, err := productContext.OwnerAuthenticatedClient().SearchProductUsersWithResponse(
				ctx, productContext.ProductID,
				ct.SearchProductUsersJSONRequestBody{},
			)
			require.NoError(t, err, "search empty product users should not error")
			assert.Equal(
				t, 200, resp.StatusCode(), "search empty product users should return 200 OK",
			)
			if assert.NotNil(t, resp.JSON200) {
				assert.Empty(t, resp.JSON200.Items, "should return empty list")
				assert.Equal(t, int64(0), resp.JSON200.Total, "total should be 0")
			}
		},
	)

	t.Run(
		"SearchProductUsersBasic", func(t *testing.T) {
			productContext := createTestProductContext(t)

			user1 := createDSLProductUser(t, productContext)
			user2 := createDSLProductUser(t, productContext)
			user3 := createDSLProductUser(t, productContext)

			resp, err := productContext.OwnerAuthenticatedClient().SearchProductUsersWithResponse(
				ctx, productContext.ProductID,
				ct.SearchProductUsersJSONRequestBody{},
			)
			require.NoError(t, err, "search product users should not error")
			assert.Equal(t, 200, resp.StatusCode(), "search product users should return 200 OK")
			if assert.NotNil(t, resp.JSON200) {
				assert.Len(t, resp.JSON200.Items, 3, "should return 3 users")
				assert.Equal(t, int64(3), resp.JSON200.Total, "total should be 3")

				userIDs := make(map[string]bool)
				for _, user := range resp.JSON200.Items {
					userIDs[user.Id] = true
				}
				assert.True(t, userIDs[user1.ID], "user1 should be in results")
				assert.True(t, userIDs[user2.ID], "user2 should be in results")
				assert.True(t, userIDs[user3.ID], "user3 should be in results")
			}
		},
	)

	t.Run(
		"SearchProductUsersWithPagination", func(t *testing.T) {
			productContext := createTestProductContext(t)

			for range 5 {
				createDSLProductUser(t, productContext)
			}

			limit := int32(2)
			offset := int32(0)
			pagination := ct.PaginationRequest{
				Limit:  &limit,
				Offset: &offset,
			}
			resp, err := productContext.OwnerAuthenticatedClient().SearchProductUsersWithResponse(
				ctx, productContext.ProductID,
				ct.SearchProductUsersJSONRequestBody{
					Pagination: &pagination,
				},
			)
			require.NoError(t, err, "search product users with pagination should not error")
			assert.Equal(
				t, 200, resp.StatusCode(),
				"search product users with pagination should return 200 OK",
			)
			if assert.NotNil(t, resp.JSON200) {
				assert.Len(t, resp.JSON200.Items, 2, "should return 2 users on first page")
				assert.Equal(t, int64(5), resp.JSON200.Total, "total should be 5")
			}

			offset = int32(2)
			pagination2 := ct.PaginationRequest{
				Limit:  &limit,
				Offset: &offset,
			}
			resp2, err := productContext.OwnerAuthenticatedClient().SearchProductUsersWithResponse(
				ctx, productContext.ProductID,
				ct.SearchProductUsersJSONRequestBody{
					Pagination: &pagination2,
				},
			)
			require.NoError(t, err, "search product users second page should not error")
			assert.Equal(
				t, 200, resp2.StatusCode(), "search product users second page should return 200 OK",
			)
			assert.NotNil(t, resp2.JSON200)
			assert.Len(t, resp2.JSON200.Items, 2, "should return 2 users on second page")
			assert.Equal(t, int64(5), resp2.JSON200.Total, "total should be 5")

			if assert.NotNil(t, resp.JSON200) && assert.NotNil(t, resp2.JSON200) {
				firstPageIDs := make(map[string]bool)
				for _, user := range resp.JSON200.Items {
					firstPageIDs[user.Id] = true
				}
				for _, user := range resp2.JSON200.Items {
					assert.False(
						t, firstPageIDs[user.Id], "second page should have different users",
					)
				}
			}
		},
	)

	t.Run(
		"SearchProductUsersByFilter", func(t *testing.T) {
			productContext := createTestProductContext(t)

			email1 := "test1@example.com"
			email2 := "test2@example.com"
			email3 := "test3@example.com"

			name := "Test User"
			status := ct.Active

			// Use API key client to create test users (create endpoint requires API key auth)
			apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:create"})

			resp1, err := apiKeyClient.CreateProductUserWithResponse(
				ctx, productContext.ProductID,
				ct.CreateProductUserJSONRequestBody{
					Email:  email1,
					Name:   &name,
					Status: &status,
				},
			)
			require.NoError(t, err)
			assert.Equal(
				t, http.StatusCreated,
				resp1.StatusCode(),
			)

			resp2, err := apiKeyClient.CreateProductUserWithResponse(
				ctx, productContext.ProductID,
				ct.CreateProductUserJSONRequestBody{
					Email:  email2,
					Name:   &name,
					Status: &status,
				},
			)
			require.NoError(t, err)
			assert.Equal(
				t, http.StatusCreated,
				resp2.StatusCode(),
			)

			resp3, err := apiKeyClient.CreateProductUserWithResponse(
				ctx, productContext.ProductID,
				ct.CreateProductUserJSONRequestBody{
					Email:  email3,
					Name:   &name,
					Status: &status,
				},
			)
			require.NoError(t, err)
			assert.Equal(
				t, http.StatusCreated,
				resp3.StatusCode(),
			)

			// Test search with Bearer token authentication (search supports both Bearer token and API key)
			searchResp, err := productContext.OwnerAuthenticatedClient().SearchProductUsersWithResponse(
				ctx, productContext.ProductID,
				ct.SearchProductUsersJSONRequestBody{},
			)
			require.NoError(t, err, "search all product users should not error")
			assert.Equal(t, 200, searchResp.StatusCode())
			if assert.NotNil(t, searchResp.JSON200) {
				assert.Len(t, searchResp.JSON200.Items, 3, "should return all 3 users")

				emailsFound := make(map[string]bool)
				for _, user := range searchResp.JSON200.Items {
					emailsFound[user.Email] = true
				}
				assert.True(t, emailsFound[email1], "should find productuserservice with email1")
				assert.True(t, emailsFound[email2], "should find productuserservice with email2")
				assert.True(t, emailsFound[email3], "should find productuserservice with email3")
			}
		},
	)
}

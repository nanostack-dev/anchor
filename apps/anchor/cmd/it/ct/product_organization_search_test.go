package ct_test

import (
	"context"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestProductOrganizationSearch(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	apiKeyClient, _ := testProduct.CreateAPIKeyClientWithAllScopes()

	org1, errInit := apiKeyClient.CreateProductOrganizationWithResponse(
		ctx,
		testProduct.ProductID,
		ct.CreateProductOrganizationJSONRequestBody{
			Name:        "Engineering Team",
			Description: new("Software engineering team"),
		},
	)
	require.NoError(t, errInit)
	require.NotNil(t, org1.JSON201)

	org2, errInit := apiKeyClient.CreateProductOrganizationWithResponse(
		ctx,
		testProduct.ProductID,
		ct.CreateProductOrganizationJSONRequestBody{
			Name:        "Marketing Team",
			Description: new("Marketing and communications team"),
		},
	)
	require.NoError(t, errInit)
	require.NotNil(t, org2.JSON201)

	org3, errInit := apiKeyClient.CreateProductOrganizationWithResponse(
		ctx,
		testProduct.ProductID,
		ct.CreateProductOrganizationJSONRequestBody{
			Name:        "Sales Team",
			Description: new("Customer sales team"),
		},
	)
	require.NoError(t, errInit)
	require.NotNil(t, org3.JSON201)

	t.Run(
		"SuccessfulSearchAllOrganizations", func(t *testing.T) {
			response, err := apiKeyClient.SearchProductOrganizationsWithResponse(
				ctx,
				testProduct.ProductID,
				nil,
				ct.SearchProductOrganizationsJSONRequestBody{
					Pagination: &ct.PaginationRequest{
						Limit:  new(int32(10)),
						Offset: new(int32(0)),
					},
				},
			)

			require.NoError(t, err, "search organizations request should not error")
			assert.Equal(t, 200, response.StatusCode(), "should return 200 OK")

			if assert.NotNil(t, response.JSON200, "response body should not be nil") {
				assert.GreaterOrEqual(
					t, response.JSON200.Count, 3, "should have at least 3 organizations",
				)
				assert.GreaterOrEqual(
					t, response.JSON200.Total, int64(3), "total should be at least 3",
				)
				assert.Len(
					t, response.JSON200.Items, response.JSON200.Count,
					"items count should match count",
				)

				if len(response.JSON200.Items) > 0 {
					firstOrg := response.JSON200.Items[0]
					assert.NotEmpty(t, firstOrg.Id, "organization ID should not be empty")
					assert.Equal(
						t, testProduct.ProductID, firstOrg.ProductId, "product ID should match",
					)
					assert.NotEmpty(t, firstOrg.Name, "organization name should not be empty")
					assert.NotEmpty(t, firstOrg.CreatedAt, "created at should not be empty")
					assert.NotEmpty(t, firstOrg.UpdatedAt, "updated at should not be empty")
				}
			}
		},
	)

	t.Run(
		"SearchOrganizationsByIDs", func(t *testing.T) {
			response, err := apiKeyClient.SearchProductOrganizationsWithResponse(
				ctx,
				testProduct.ProductID,
				nil,
				ct.SearchProductOrganizationsJSONRequestBody{
					Filter: &ct.OrganizationFilter{
						Ids: []ct.Ksuid{org1.JSON201.Id, org2.JSON201.Id},
					},
					Pagination: &ct.PaginationRequest{
						Limit:  new(int32(10)),
						Offset: new(int32(0)),
					},
				},
			)

			require.NoError(t, err, "search organizations by IDs should not error")
			assert.Equal(t, 200, response.StatusCode(), "should return 200 OK")

			if assert.NotNil(t, response.JSON200, "response body should not be nil") {
				assert.Equal(t, 2, response.JSON200.Count, "should return exactly 2 organizations")
				assert.Equal(t, int64(2), response.JSON200.Total, "total should be 2")

				foundIDs := make(map[string]bool)
				for _, org := range response.JSON200.Items {
					foundIDs[org.Id] = true
				}
				assert.True(t, foundIDs[org1.JSON201.Id], "should find first organization")
				assert.True(t, foundIDs[org2.JSON201.Id], "should find second organization")
				assert.False(t, foundIDs[org3.JSON201.Id], "should not find third organization")
			}
		},
	)

	t.Run(
		"SearchOrganizationsByNames", func(t *testing.T) {
			response, err := apiKeyClient.SearchProductOrganizationsWithResponse(
				ctx,
				testProduct.ProductID,
				nil,
				ct.SearchProductOrganizationsJSONRequestBody{
					Filter: &ct.OrganizationFilter{
						Names: []string{"Engineering Team", "Sales Team"},
					},
					Pagination: &ct.PaginationRequest{
						Limit:  new(int32(10)),
						Offset: new(int32(0)),
					},
				},
			)

			require.NoError(t, err, "search organizations by names should not error")
			assert.Equal(t, 200, response.StatusCode(), "should return 200 OK")

			if assert.NotNil(t, response.JSON200, "response body should not be nil") {
				assert.Equal(t, 2, response.JSON200.Count, "should return exactly 2 organizations")
				assert.Equal(t, int64(2), response.JSON200.Total, "total should be 2")

				foundNames := make(map[string]bool)
				for _, org := range response.JSON200.Items {
					foundNames[org.Name] = true
				}
				assert.True(t, foundNames["Engineering Team"], "should find Engineering Team")
				assert.True(t, foundNames["Sales Team"], "should find Sales Team")
				assert.False(t, foundNames["Marketing Team"], "should not find Marketing Team")
			}
		},
	)

	t.Run(
		"SearchOrganizationsWithFullTextSearch", func(t *testing.T) {
			response, err := apiKeyClient.SearchProductOrganizationsWithResponse(
				ctx,
				testProduct.ProductID,
				nil,
				ct.SearchProductOrganizationsJSONRequestBody{
					FullTextSearch: new("Team"),
					Pagination: &ct.PaginationRequest{
						Limit:  new(int32(10)),
						Offset: new(int32(0)),
					},
				},
			)

			require.NoError(t, err, "search organizations with full text should not error")
			assert.Equal(t, 200, response.StatusCode(), "should return 200 OK")

			if assert.NotNil(t, response.JSON200, "response body should not be nil") {
				assert.GreaterOrEqual(t, response.JSON200.Count, 3, "should find all teams")
				assert.GreaterOrEqual(
					t, response.JSON200.Total, int64(3), "total should be at least 3",
				)

				for _, org := range response.JSON200.Items {
					assert.Contains(t, org.Name, "Team", "organization name should contain 'Team'")
				}
			}
		},
	)

	t.Run(
		"SearchOrganizationsWithSorting", func(t *testing.T) {
			response, err := apiKeyClient.SearchProductOrganizationsWithResponse(
				ctx,
				testProduct.ProductID,
				nil,
				ct.SearchProductOrganizationsJSONRequestBody{
					SortBy:        ptr.Ptr(ct.ProductOrganizationSearchRequestSortByName),
					SortDirection: ptr.Ptr(ct.ASC),
					Pagination: &ct.PaginationRequest{
						Limit:  new(int32(10)),
						Offset: new(int32(0)),
					},
				},
			)

			require.NoError(t, err, "search organizations with sorting should not error")
			assert.Equal(t, 200, response.StatusCode(), "should return 200 OK")

			if assert.NotNil(t, response.JSON200, "response body should not be nil") {
				assert.GreaterOrEqual(
					t, response.JSON200.Count, 3, "should have at least 3 organizations",
				)

				if len(response.JSON200.Items) >= 3 {
					orgNames := make([]string, 0, len(response.JSON200.Items))
					for _, org := range response.JSON200.Items {
						orgNames = append(orgNames, org.Name)
					}

					for i := 1; i < len(orgNames); i++ {
						assert.LessOrEqual(
							t, orgNames[i-1], orgNames[i], "organizations should be sorted by name",
						)
					}
				}
			}
		},
	)

	t.Run(
		"SearchOrganizationsWithPagination", func(t *testing.T) {
			response1, err := apiKeyClient.SearchProductOrganizationsWithResponse(
				ctx,
				testProduct.ProductID,
				nil,
				ct.SearchProductOrganizationsJSONRequestBody{
					SortBy:        ptr.Ptr(ct.ProductOrganizationSearchRequestSortByName),
					SortDirection: ptr.Ptr(ct.ASC),
					Pagination: &ct.PaginationRequest{
						Limit:  new(int32(2)),
						Offset: new(int32(0)),
					},
				},
			)

			require.NoError(t, err, "search first page should not error")
			assert.Equal(t, 200, response1.StatusCode(), "should return 200 OK")

			response2, err := apiKeyClient.SearchProductOrganizationsWithResponse(
				ctx,
				testProduct.ProductID,
				nil,
				ct.SearchProductOrganizationsJSONRequestBody{
					SortBy:        ptr.Ptr(ct.ProductOrganizationSearchRequestSortByName),
					SortDirection: ptr.Ptr(ct.ASC),
					Pagination: &ct.PaginationRequest{
						Limit:  new(int32(2)),
						Offset: new(int32(2)),
					},
				},
			)

			require.NoError(t, err, "search second page should not error")
			assert.Equal(t, 200, response2.StatusCode(), "should return 200 OK")

			if assert.NotNil(t, response1.JSON200, "first page response should not be nil") &&
				assert.NotNil(t, response2.JSON200, "second page response should not be nil") {
				assert.Equal(
					t, response1.JSON200.Total, response2.JSON200.Total, "total should be the same",
				)

				assert.Equal(t, 2, response1.JSON200.Count, "first page should have 2 items")

				firstPageIDs := make(map[string]bool)
				for _, org := range response1.JSON200.Items {
					firstPageIDs[org.Id] = true
				}

				for _, org := range response2.JSON200.Items {
					assert.False(
						t, firstPageIDs[org.Id],
						"second page should not contain items from first page",
					)
				}
			}
		},
	)

	t.Run(
		"SearchOrganizationsEmptyResults", func(t *testing.T) {
			response, err := apiKeyClient.SearchProductOrganizationsWithResponse(
				ctx,
				testProduct.ProductID,
				nil,
				ct.SearchProductOrganizationsJSONRequestBody{
					Filter: &ct.OrganizationFilter{
						Names: []string{"Non-existent Organization"},
					},
					Pagination: &ct.PaginationRequest{
						Limit:  new(int32(10)),
						Offset: new(int32(0)),
					},
				},
			)

			require.NoError(t, err, "search non-existent organizations should not error")
			assert.Equal(t, 200, response.StatusCode(), "should return 200 OK")

			if assert.NotNil(t, response.JSON200, "response body should not be nil") {
				assert.Equal(t, 0, response.JSON200.Count, "should return 0 organizations")
				assert.Equal(t, int64(0), response.JSON200.Total, "total should be 0")
				assert.Empty(t, response.JSON200.Items, "items should be empty")
			}
		},
	)

	t.Run(
		"SearchOrganizationsInvalidPagination", func(t *testing.T) {
			response, err := apiKeyClient.SearchProductOrganizationsWithResponse(
				ctx,
				testProduct.ProductID,
				nil,
				ct.SearchProductOrganizationsJSONRequestBody{
					Pagination: &ct.PaginationRequest{
						Limit:  new(int32(-1)), // Invalid limit
						Offset: new(int32(0)),
					},
				},
			)

			require.NoError(t, err, "search with invalid pagination should not error")
			assert.Equal(t, 400, response.StatusCode(), "should return 400 Bad Request")

			if assert.NotNil(t, response.JSON400, "error response should not be nil") {
				assert.Contains(t, response.JSON400.Errors[0].Code, "VALIDATION_ERROR")
				assert.Contains(t, response.JSON400.Errors[0].Message, "Limit")
			}
		},
	)

	t.Run(
		"SearchOrganizationsNonExistentProduct", func(t *testing.T) {
			nonExistentProductID := "prod_nonexistent123"

			response, err := apiKeyClient.SearchProductOrganizationsWithResponse(
				ctx,
				nonExistentProductID,
				nil,
				ct.SearchProductOrganizationsJSONRequestBody{
					Pagination: &ct.PaginationRequest{
						Limit:  new(int32(10)),
						Offset: new(int32(0)),
					},
				},
			)

			require.NoError(
				t, err, "search organizations for non-existent product should not error",
			)
			assert.True(
				t, response.StatusCode() == 404 || response.StatusCode() == 400,
				"should return 404 or 400 for non-existent product",
			)
		},
	)

	t.Run(
		"SecuritySearchOrganizationsWithInvalidPermissions", func(t *testing.T) {
			apiKeyClientWithBadPermissions, apiKeyIDBadPerm := testProduct.CreateAPIKeyClientWithScopes(
				[]string{"organization:create"},
			)

			response, err := apiKeyClientWithBadPermissions.SearchProductOrganizationsWithResponse(
				ctx,
				testProduct.ProductID,
				nil,
				ct.SearchProductOrganizationsJSONRequestBody{
					Pagination: &ct.PaginationRequest{
						Limit:  new(int32(10)),
						Offset: new(int32(0)),
					},
				},
			)

			require.NoError(t, err, "search organizations request should not error")
			itshared.AssertProductAPIKeyInsufficientPermissions(
				t,
				response,
				apiKeyIDBadPerm,
				[]string{"organization:read"},
				[]string{"organization:create"},
			)
		},
	)
}

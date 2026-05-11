package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	itshared "anchor/cmd/it/shared"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationAPIKeySearch(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	apiKeyClient, _ := product.CreateAPIKeyClientWithScopes([]string{
		"organization_api_key:create",
		"organization_api_key:read",
	})
	description := itshared.Faker.Lorem().Sentence(4)
	org := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description)
	permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
	name1 := "SearchAlpha-" + uuid.NewString()
	name2 := "SearchBeta-" + uuid.NewString()

	_, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
		ctx,
		product.ProductID,
		org.Id,
		ct.CreateOrganizationAPIKeyJSONRequestBody{Name: name1, Permissions: []string{permissions.FileRead}},
	)
	require.NoError(t, err)
	_, err = apiKeyClient.CreateOrganizationAPIKeyWithResponse(
		ctx,
		product.ProductID,
		org.Id,
		ct.CreateOrganizationAPIKeyJSONRequestBody{Name: name2, Permissions: []string{permissions.FileRead}},
	)
	require.NoError(t, err)

	t.Run("Search organization API keys", func(t *testing.T) {
		searchResp, searchErr := apiKeyClient.SearchOrganizationAPIKeysWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			ct.SearchOrganizationAPIKeysJSONRequestBody{
				Filter: &ct.OrganizationAPIKeyFilter{Names: []string{name1}},
			},
		)
		require.NoError(t, searchErr)
		require.Equal(t, http.StatusOK, searchResp.StatusCode())
		require.NotNil(t, searchResp.JSON200)
		require.Len(t, searchResp.JSON200.Items, 1)
		assert.Equal(t, name1, searchResp.JSON200.Items[0].Name)
	})

	t.Run("Search returns inactive status after expiration queue processing", func(t *testing.T) {
		expiresAt := time.Now().UTC().Add(1 * time.Second).Truncate(time.Second)
		expiredName := "SearchExpired-" + uuid.NewString()

		_, createErr := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			ct.CreateOrganizationAPIKeyJSONRequestBody{
				Name:        expiredName,
				ExpiresAt:   &expiresAt,
				Permissions: []string{permissions.FileRead},
			},
		)
		require.NoError(t, createErr)

		require.Eventually(t, func() bool {
			searchResp, searchErr := apiKeyClient.SearchOrganizationAPIKeysWithResponse(
				ctx,
				product.ProductID,
				org.Id,
				ct.SearchOrganizationAPIKeysJSONRequestBody{
					Filter: &ct.OrganizationAPIKeyFilter{Names: []string{expiredName}},
				},
			)
			require.NoError(t, searchErr)
			require.Equal(t, http.StatusOK, searchResp.StatusCode())
			require.NotNil(t, searchResp.JSON200)
			require.Len(t, searchResp.JSON200.Items, 1)
			return searchResp.JSON200.Items[0].Status == ct.OrganizationAPIKeyStatusINACTIVE
		}, 10*time.Second, 500*time.Millisecond)
	})
}

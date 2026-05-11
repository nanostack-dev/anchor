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

func TestOrganizationAPIKeyGet(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	apiKeyClient, _ := product.CreateAPIKeyClientWithScopes([]string{
		"organization_api_key:create",
		"organization_api_key:read",
	})
	description := itshared.Faker.Lorem().Sentence(4)
	org := product.CreateOrganization(t, "Org-"+uuid.NewString(), &description)
	permissions := givenOrganizationAPIKeyResourcePermissions(t, product)

	createResp, err := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
		ctx,
		product.ProductID,
		org.Id,
		ct.CreateOrganizationAPIKeyJSONRequestBody{
			Name:        "GetKey-" + uuid.NewString(),
			Permissions: []string{permissions.FileRead},
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode())

	t.Run("Get organization API key by organization and id", func(t *testing.T) {
		getResp, getErr := apiKeyClient.GetOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			createResp.JSON201.Id,
		)
		require.NoError(t, getErr)
		require.Equal(t, http.StatusOK, getResp.StatusCode())
		require.NotNil(t, getResp.JSON200)
		assert.Equal(t, createResp.JSON201.Id, getResp.JSON200.Id)
		assert.Equal(t, org.Id, getResp.JSON200.OrganizationId)
	})

	t.Run("Get with wrong organization returns not found", func(t *testing.T) {
		otherDescription := itshared.Faker.Lorem().Sentence(4)
		otherOrg := product.CreateOrganization(t, "Org-"+uuid.NewString(), &otherDescription)

		getResp, getErr := apiKeyClient.GetOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			otherOrg.Id,
			createResp.JSON201.Id,
		)
		require.NoError(t, getErr)
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode())
	})

	t.Run("Get expired organization API key returns inactive status after queue processing", func(t *testing.T) {
		expiresAt := time.Now().UTC().Add(1 * time.Second).Truncate(time.Second)
		expiredResp, expiredErr := apiKeyClient.CreateOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			ct.CreateOrganizationAPIKeyJSONRequestBody{
				Name:        "ExpiredGetKey-" + uuid.NewString(),
				ExpiresAt:   &expiresAt,
				Permissions: []string{permissions.FileRead},
			},
		)
		require.NoError(t, expiredErr)
		require.Equal(t, http.StatusCreated, expiredResp.StatusCode())

		require.Eventually(t, func() bool {
			getResp, getErr := apiKeyClient.GetOrganizationAPIKeyWithResponse(
				ctx,
				product.ProductID,
				org.Id,
				expiredResp.JSON201.Id,
			)
			require.NoError(t, getErr)
			require.Equal(t, http.StatusOK, getResp.StatusCode())
			require.NotNil(t, getResp.JSON200)
			return getResp.JSON200.Status == ct.OrganizationAPIKeyStatusINACTIVE
		}, 10*time.Second, 500*time.Millisecond)
	})
}

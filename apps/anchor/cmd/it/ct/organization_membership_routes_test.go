package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationMembershipRoutes(t *testing.T) {
	ctx := context.Background()
	productCtx := createTestProductContext(t)
	apiKeyClient, _ := productCtx.CreateAPIKeyClientWithAllScopes()

	role := createDSLProductRole(t, productCtx, "Org Member Role", nil)
	orgResp, err := apiKeyClient.CreateProductOrganizationWithResponse(
		ctx,
		productCtx.ProductID,
		ct.CreateProductOrganizationJSONRequestBody{Name: "Org Members CT"},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, orgResp.StatusCode())
	require.NotNil(t, orgResp.JSON201)

	t.Run("AddMemberAndDuplicateReturnsConflict", func(t *testing.T) {
		productUser := createDSLProductUser(t, productCtx)

		addResp, addErr := apiKeyClient.AddOrganizationMemberWithResponse(
			ctx,
			productCtx.ProductID,
			orgResp.JSON201.Id,
			ct.AddOrganizationMemberJSONRequestBody{
				ProductUserId: productUser.ID,
				RoleId:        role.ID,
			},
		)
		require.NoError(t, addErr)
		require.Equal(t, http.StatusCreated, addResp.StatusCode())
		require.NotNil(t, addResp.JSON201)
		assert.Equal(t, productUser.ID, addResp.JSON201.ProductUserId)

		dupResp, dupErr := apiKeyClient.AddOrganizationMemberWithResponse(
			ctx,
			productCtx.ProductID,
			orgResp.JSON201.Id,
			ct.AddOrganizationMemberJSONRequestBody{
				ProductUserId: productUser.ID,
				RoleId:        role.ID,
			},
		)
		require.NoError(t, dupErr)
		require.Equal(t, http.StatusConflict, dupResp.StatusCode())

		var errResp ct.ApiErrorResponse
		require.NoError(t, json.Unmarshal(dupResp.Body, &errResp))
		require.NotEmpty(t, errResp.Errors)
		assert.Equal(t, "ORGANIZATION_MEMBERSHIP_ALREADY_EXISTS", errResp.Errors[0].Code)
	})

	t.Run("AddMemberWithNonExistentProductUserReturnsNotFound", func(t *testing.T) {
		addResp, addErr := apiKeyClient.AddOrganizationMemberWithResponse(
			ctx,
			productCtx.ProductID,
			orgResp.JSON201.Id,
			ct.AddOrganizationMemberJSONRequestBody{
				ProductUserId: ids.MustNew("pusr"),
				RoleId:        role.ID,
			},
		)
		require.NoError(t, addErr)
		// Previously this hit a FK violation and returned 500.
		assert.Equal(t, http.StatusNotFound, addResp.StatusCode())

		var errResp ct.ApiErrorResponse
		require.NoError(t, json.Unmarshal(addResp.Body, &errResp))
		require.NotEmpty(t, errResp.Errors)
		assert.Equal(t, "PRODUCT_USER_NOT_FOUND", errResp.Errors[0].Code)
	})

	t.Run("SearchMembersByExternalID", func(t *testing.T) {
		productUser := createDSLProductUser(t, productCtx)
		externalID := "ext-search-member-1"
		setDSLProductUserExternalID(t, productCtx, productUser.ID, externalID)

		_, addErr := apiKeyClient.AddOrganizationMemberWithResponse(
			ctx,
			productCtx.ProductID,
			orgResp.JSON201.Id,
			ct.AddOrganizationMemberJSONRequestBody{
				ProductUserId: productUser.ID,
				RoleId:        role.ID,
			},
		)
		require.NoError(t, addErr)

		searchResp, searchErr := apiKeyClient.SearchOrganizationMembersWithResponse(
			ctx,
			productCtx.ProductID,
			orgResp.JSON201.Id,
			ct.SearchOrganizationMembersJSONRequestBody{
				Filter: &ct.OrganizationMemberFilter{
					ExternalIds: &[]string{externalID},
				},
			},
		)
		require.NoError(t, searchErr)
		require.Equal(t, http.StatusOK, searchResp.StatusCode())
		require.NotNil(t, searchResp.JSON200)
		require.NotEmpty(t, searchResp.JSON200.Items)
		assert.Equal(t, productUser.ID, searchResp.JSON200.Items[0].ProductUserId)
		require.NotNil(t, searchResp.JSON200.Items[0].ExternalId)
		assert.Equal(t, externalID, *searchResp.JSON200.Items[0].ExternalId)
	})

	t.Run("CreateOrganizationWithMemberIsIdempotent", func(t *testing.T) {
		productUser := createDSLProductUser(t, productCtx)
		requestBody := ct.CreateProductOrganizationJSONRequestBody{Name: "Atomic Create CT"}
		requestBody.FoundingMember = &ct.FoundingMemberRequest{
			ProductUserId: productUser.ID,
			RoleId:        role.ID,
		}

		firstResp, firstErr := apiKeyClient.CreateProductOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			requestBody,
		)
		require.NoError(t, firstErr)
		require.Equal(t, http.StatusCreated, firstResp.StatusCode())
		require.NotNil(t, firstResp.JSON201)

		secondResp, secondErr := apiKeyClient.CreateProductOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			requestBody,
		)
		require.NoError(t, secondErr)
		require.Equal(t, http.StatusCreated, secondResp.StatusCode())
		require.NotNil(t, secondResp.JSON201)

		assert.Equal(t, firstResp.JSON201.Id, secondResp.JSON201.Id)
	})
}

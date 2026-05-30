package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestOrganizationWorkspaceRoutes(t *testing.T) {
	ctx := context.Background()
	productCtx := createTestProductContext(t)
	apiKeyClient, _ := productCtx.CreateAPIKeyClientWithAllScopes()
	organization := productCtx.CreateOrganization(
		t,
		"workspace-org-"+itshared.Faker.UUID().V4(),
		ptr.Ptr("Workspace route test organization"),
	)

	t.Run("CreateGetSearchUpdateAndDeleteWorkspace", func(t *testing.T) {
		workspaceName := "workspace-" + itshared.Faker.UUID().V4()
		created := createWorkspace(t, apiKeyClient, productCtx.ProductID, organization.Id, workspaceName)

		assert.Equal(t, organization.Id, created.OrganizationId)
		assert.Equal(t, workspaceName, created.Name)
		assert.NotEmpty(t, created.Id)
		assert.NotEmpty(t, created.CreatedAt)
		assert.NotEmpty(t, created.UpdatedAt)

		getResp, err := apiKeyClient.GetOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			created.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, getResp.StatusCode())
		require.NotNil(t, getResp.JSON200)
		assert.Equal(t, created.Id, getResp.JSON200.Id)

		sortBy := ct.ProductWorkspaceSearchRequestSortByName
		sortDirection := ct.ASC
		searchResp, err := apiKeyClient.SearchOrganizationWorkspacesWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			ct.SearchOrganizationWorkspacesJSONRequestBody{
				Filter: &ct.WorkspaceFilter{
					Ids:   []string{created.Id},
					Names: []string{created.Name},
				},
				FullTextSearch: ptr.Ptr(created.Name),
				Pagination: &ct.PaginationRequest{
					Limit:  ptr.Ptr(int32(10)),
					Offset: ptr.Ptr(int32(0)),
				},
				SortBy:        &sortBy,
				SortDirection: &sortDirection,
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, searchResp.StatusCode())
		require.NotNil(t, searchResp.JSON200)
		assert.Equal(t, 1, searchResp.JSON200.Count)
		assert.Equal(t, created.Id, searchResp.JSON200.Items[0].Id)

		updatedName := workspaceName + "-updated"
		updateResp, err := apiKeyClient.UpdateOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			created.Id,
			ct.UpdateOrganizationWorkspaceJSONRequestBody{
				Name:        updatedName,
				Description: ptr.Ptr("Updated workspace description"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, updateResp.StatusCode())
		require.NotNil(t, updateResp.JSON200)
		assert.Equal(t, updatedName, updateResp.JSON200.Name)
		assert.Equal(t, "Updated workspace description", *updateResp.JSON200.Description)
		assert.True(t, updateResp.JSON200.UpdatedAt.After(created.UpdatedAt))

		deleteResp, err := apiKeyClient.DeleteOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			created.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode())

		getDeletedResp, err := apiKeyClient.GetOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			created.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, getDeletedResp.StatusCode())
	})

	t.Run("DuplicateNameRejectedWithinOrganizationButAllowedAcrossOrganizations", func(t *testing.T) {
		workspaceName := "duplicate-workspace-" + itshared.Faker.UUID().V4()
		created := createWorkspace(t, apiKeyClient, productCtx.ProductID, organization.Id, workspaceName)

		duplicateResp, err := apiKeyClient.CreateOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			ct.CreateOrganizationWorkspaceJSONRequestBody{Name: workspaceName},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, duplicateResp.StatusCode())
		require.NotNil(t, duplicateResp.JSON400)
		assert.Equal(t, "WORKSPACE_NAME_DUPLICATE", duplicateResp.JSON400.Errors[0].Code)

		otherOrganization := productCtx.CreateOrganization(
			t,
			"workspace-org-"+itshared.Faker.UUID().V4(),
			nil,
		)
		sameNameOtherOrg := createWorkspace(
			t,
			apiKeyClient,
			productCtx.ProductID,
			otherOrganization.Id,
			workspaceName,
		)
		assert.Equal(t, workspaceName, sameNameOtherOrg.Name)
		assert.NotEqual(t, created.Id, sameNameOtherOrg.Id)
	})

	t.Run("UpdateDuplicateNameRejected", func(t *testing.T) {
		first := createWorkspace(
			t,
			apiKeyClient,
			productCtx.ProductID,
			organization.Id,
			"first-workspace-"+itshared.Faker.UUID().V4(),
		)
		second := createWorkspace(
			t,
			apiKeyClient,
			productCtx.ProductID,
			organization.Id,
			"second-workspace-"+itshared.Faker.UUID().V4(),
		)

		updateResp, err := apiKeyClient.UpdateOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			first.Id,
			ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: second.Name},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, updateResp.StatusCode())
		require.NotNil(t, updateResp.JSON400)
		assert.Equal(t, "WORKSPACE_NAME_DUPLICATE", updateResp.JSON400.Errors[0].Code)
	})

	t.Run("WorkspaceIsolationByOrganizationAndProduct", func(t *testing.T) {
		workspace := createWorkspace(
			t,
			apiKeyClient,
			productCtx.ProductID,
			organization.Id,
			"isolated-workspace-"+itshared.Faker.UUID().V4(),
		)
		otherOrganization := productCtx.CreateOrganization(
			t,
			"workspace-org-"+itshared.Faker.UUID().V4(),
			nil,
		)

		wrongOrgResp, err := apiKeyClient.GetOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			otherOrganization.Id,
			workspace.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, wrongOrgResp.StatusCode())

		otherProductCtx := createTestProductContext(t)
		otherProductClient, _ := otherProductCtx.CreateAPIKeyClientWithAllScopes()
		wrongProductResp, err := otherProductClient.GetOrganizationWorkspaceWithResponse(
			ctx,
			otherProductCtx.ProductID,
			organization.Id,
			workspace.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, wrongProductResp.StatusCode())

		wrongOrgUpdateResp, err := apiKeyClient.UpdateOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			otherOrganization.Id,
			workspace.Id,
			ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: workspace.Name + "-wrong-org"},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, wrongOrgUpdateResp.StatusCode())

		wrongProductDeleteResp, err := otherProductClient.DeleteOrganizationWorkspaceWithResponse(
			ctx,
			otherProductCtx.ProductID,
			organization.Id,
			workspace.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, wrongProductDeleteResp.StatusCode())

		stillFoundResp, err := apiKeyClient.GetOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			workspace.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, stillFoundResp.StatusCode())
		require.NotNil(t, stillFoundResp.JSON200)
		assert.Equal(t, workspace.Name, stillFoundResp.JSON200.Name)

		searchWrongOrgResp, err := apiKeyClient.SearchOrganizationWorkspacesWithResponse(
			ctx,
			productCtx.ProductID,
			otherOrganization.Id,
			ct.SearchOrganizationWorkspacesJSONRequestBody{
				Filter: &ct.WorkspaceFilter{Ids: []string{workspace.Id}},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, searchWrongOrgResp.StatusCode())
		require.NotNil(t, searchWrongOrgResp.JSON200)
		assert.Empty(t, searchWrongOrgResp.JSON200.Items)
	})
}

func TestOrganizationWorkspaceAuthorization(t *testing.T) {
	ctx := context.Background()
	productCtx := createTestProductContext(t)
	apiKeyClient, _ := productCtx.CreateAPIKeyClientWithAllScopes()
	organization := productCtx.CreateOrganization(
		t,
		"workspace-auth-org-"+itshared.Faker.UUID().V4(),
		nil,
	)
	workspace := createWorkspace(
		t,
		apiKeyClient,
		productCtx.ProductID,
		organization.Id,
		"auth-workspace-"+itshared.Faker.UUID().V4(),
	)

	t.Run("PlatformAdminCanReadButCannotMutate", func(t *testing.T) {
		ownerClient := productCtx.OwnerAuthenticatedClient()

		getResp, err := ownerClient.GetOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			workspace.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, getResp.StatusCode())
		require.NotNil(t, getResp.JSON200)

		searchResp, err := ownerClient.SearchOrganizationWorkspacesWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			ct.SearchOrganizationWorkspacesJSONRequestBody{
				Pagination: &ct.PaginationRequest{
					Limit:  ptr.Ptr(int32(10)),
					Offset: ptr.Ptr(int32(0)),
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, searchResp.StatusCode())
		require.NotNil(t, searchResp.JSON200)

		createResp, err := ownerClient.CreateOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "platform-mutating-workspace"},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, createResp.StatusCode())

		updateResp, err := ownerClient.UpdateOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			workspace.Id,
			ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: workspace.Name + "-platform"},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, updateResp.StatusCode())

		deleteResp, err := ownerClient.DeleteOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			workspace.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, deleteResp.StatusCode())
	})

	t.Run("ProductAPIKeyRequiresWorkspaceScopes", func(t *testing.T) {
		readOnlyClient, readOnlyKeyID := productCtx.CreateAPIKeyClientWithScopes([]string{"workspace:read"})
		createResp, err := readOnlyClient.CreateOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "forbidden-create-workspace"},
		)
		require.NoError(t, err)
		itshared.AssertProductAPIKeyInsufficientPermissions(
			t,
			createResp,
			readOnlyKeyID,
			[]string{"workspace:create"},
			[]string{"workspace:read"},
		)

		createOnlyClient, createOnlyKeyID := productCtx.CreateAPIKeyClientWithScopes([]string{"workspace:create"})
		searchResp, err := createOnlyClient.SearchOrganizationWorkspacesWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			ct.SearchOrganizationWorkspacesJSONRequestBody{},
		)
		require.NoError(t, err)
		itshared.AssertProductAPIKeyInsufficientPermissions(
			t,
			searchResp,
			createOnlyKeyID,
			[]string{"workspace:read"},
			[]string{"workspace:create"},
		)

		updateResp, err := readOnlyClient.UpdateOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			workspace.Id,
			ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: workspace.Name + "-forbidden"},
		)
		require.NoError(t, err)
		itshared.AssertProductAPIKeyInsufficientPermissions(
			t,
			updateResp,
			readOnlyKeyID,
			[]string{"workspace:update"},
			[]string{"workspace:read"},
		)

		deleteResp, err := readOnlyClient.DeleteOrganizationWorkspaceWithResponse(
			ctx,
			productCtx.ProductID,
			organization.Id,
			workspace.Id,
		)
		require.NoError(t, err)
		itshared.AssertProductAPIKeyInsufficientPermissions(
			t,
			deleteResp,
			readOnlyKeyID,
			[]string{"workspace:delete"},
			[]string{"workspace:read"},
		)
	})
}

func createWorkspace(
	t *testing.T,
	client *ct.ClientWithResponses,
	productID string,
	organizationID string,
	name string,
) ct.ProductWorkspaceResponse {
	t.Helper()

	resp, err := client.CreateOrganizationWorkspaceWithResponse(
		context.Background(),
		productID,
		organizationID,
		ct.CreateOrganizationWorkspaceJSONRequestBody{
			Name:        name,
			Description: ptr.Ptr("Workspace test description"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode())
	require.NotNil(t, resp.JSON201)

	return *resp.JSON201
}

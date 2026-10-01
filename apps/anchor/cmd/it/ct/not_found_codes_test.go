package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireNotFoundCode(t *testing.T, status int, body []byte, code string) {
	t.Helper()
	require.Equal(t, http.StatusNotFound, status, string(body))
	assert.Equal(t, code, decodeAPIError(t, body).Errors[0].Code)
}

func TestNotFoundCodes_Organization(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	productCtx := createTestProductContext(t)
	client, _ := productCtx.CreateAPIKeyClientWithAllScopes()
	role := createDSLProductRole(t, productCtx, "Missing Org Role", nil)
	user := createDSLProductUser(t, productCtx)
	missingOrg := ids.MustNew("org")

	t.Run("Get", func(t *testing.T) {
		resp, err := client.GetProductOrganizationWithResponse(ctx, productCtx.ProductID, missingOrg, nil)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "ORGANIZATION_NOT_FOUND")
	})
	t.Run("Update", func(t *testing.T) {
		resp, err := client.UpdateProductOrganizationWithResponse(
			ctx, productCtx.ProductID, missingOrg, ct.UpdateProductOrganizationJSONRequestBody{Name: "Missing"},
		)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "ORGANIZATION_NOT_FOUND")
	})
	t.Run("Delete", func(t *testing.T) {
		resp, err := client.DeleteProductOrganizationWithResponse(ctx, productCtx.ProductID, missingOrg)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "ORGANIZATION_NOT_FOUND")
	})
	t.Run("AddMember", func(t *testing.T) {
		resp, err := client.AddOrganizationMemberWithResponse(
			ctx, productCtx.ProductID, missingOrg,
			ct.AddOrganizationMemberJSONRequestBody{ProductUserId: user.ID, RoleId: role.ID},
		)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "ORGANIZATION_NOT_FOUND")
	})
	t.Run("CreateWorkspace", func(t *testing.T) {
		resp, err := client.CreateOrganizationWorkspaceWithResponse(
			ctx, productCtx.ProductID, missingOrg, ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Missing Org"},
		)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "ORGANIZATION_NOT_FOUND")
	})
	t.Run("SearchWorkspaces", func(t *testing.T) {
		resp, err := client.SearchOrganizationWorkspacesWithResponse(
			ctx, productCtx.ProductID, missingOrg, ct.SearchOrganizationWorkspacesJSONRequestBody{},
		)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "ORGANIZATION_NOT_FOUND")
	})
}

func TestNotFoundCodes_OrganizationMembership(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	productCtx := createTestProductContext(t)
	client, _ := productCtx.CreateAPIKeyClientWithAllScopes()
	role := createDSLProductRole(t, productCtx, "Missing Member Role", nil)
	org := productCtx.CreateOrganization(t, "Missing Member Org", nil)
	nonMember := createDSLProductUser(t, productCtx)

	t.Run("Get", func(t *testing.T) {
		resp, err := client.GetOrganizationMemberWithResponse(ctx, productCtx.ProductID, org.Id, nonMember.ID, nil)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "ORGANIZATION_MEMBERSHIP_NOT_FOUND")
	})
	t.Run("UpdateRole", func(t *testing.T) {
		resp, err := client.UpdateOrganizationMemberRoleWithResponse(
			ctx, productCtx.ProductID, org.Id, nonMember.ID,
			ct.UpdateOrganizationMemberRoleJSONRequestBody{RoleId: role.ID},
		)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "ORGANIZATION_MEMBERSHIP_NOT_FOUND")
	})
	t.Run("Remove", func(t *testing.T) {
		resp, err := client.RemoveOrganizationMemberWithResponse(ctx, productCtx.ProductID, org.Id, nonMember.ID)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "ORGANIZATION_MEMBERSHIP_NOT_FOUND")
	})
}

func TestNotFoundCodes_Workspace(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	productCtx := createTestProductContext(t)
	client, _ := productCtx.CreateAPIKeyClientWithAllScopes()
	org := productCtx.CreateOrganization(t, "Missing Workspace Org", nil)
	missingWorkspace := ids.MustNew("wsp")

	t.Run("Get", func(t *testing.T) {
		resp, err := client.GetOrganizationWorkspaceWithResponse(ctx, productCtx.ProductID, org.Id, missingWorkspace)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "WORKSPACE_NOT_FOUND")
	})
	t.Run("Update", func(t *testing.T) {
		resp, err := client.UpdateOrganizationWorkspaceWithResponse(
			ctx, productCtx.ProductID, org.Id, missingWorkspace,
			ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: "Missing"},
		)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "WORKSPACE_NOT_FOUND")
	})
	t.Run("Delete", func(t *testing.T) {
		resp, err := client.DeleteOrganizationWorkspaceWithResponse(ctx, productCtx.ProductID, org.Id, missingWorkspace)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "WORKSPACE_NOT_FOUND")
	})
}

func TestNotFoundCodes_ProductResourcePermission(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	productCtx := createTestProductContext(t)
	client, _ := productCtx.CreateAPIKeyClientWithAllScopes()
	const missingName = "missing:permission"

	t.Run("Get", func(t *testing.T) {
		resp, err := client.GetProductResourcePermissionWithResponse(ctx, productCtx.ProductID, missingName)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "RESOURCE_PERMISSION_NOT_FOUND")
	})
	t.Run("Update", func(t *testing.T) {
		resp, err := client.UpdateProductResourcePermissionWithResponse(
			ctx, productCtx.ProductID, missingName,
			ct.UpdateProductResourcePermissionRequest{Description: new("Missing")},
		)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "RESOURCE_PERMISSION_NOT_FOUND")
	})
	t.Run("Delete", func(t *testing.T) {
		resp, err := client.DeleteProductResourcePermissionWithResponse(ctx, productCtx.ProductID, missingName)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "RESOURCE_PERMISSION_NOT_FOUND")
	})
}

func TestNotFoundCodes_ProductPermission(t *testing.T) {
	t.Parallel()
	productCtx := createTestProductContext(t)

	resp, err := productCtx.OwnerAuthenticatedClient().GetProductPermissionWithResponse(
		context.Background(), productCtx.ProductID, "missing:permission",
	)

	require.NoError(t, err)
	requireNotFoundCode(t, resp.StatusCode(), resp.Body, "PERMISSION_NOT_FOUND")
}

func TestNotFoundCodes_ProductAPIKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	productCtx := createTestProductContext(t)
	client := productCtx.OwnerAuthenticatedClient()
	missingKey := ids.MustNew("product_apikey")

	t.Run("Get", func(t *testing.T) {
		resp, err := client.GetProductAPIKeyWithResponse(ctx, productCtx.ProductID, missingKey)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "PRODUCT_API_KEY_NOT_FOUND")
	})
	t.Run("Update", func(t *testing.T) {
		resp, err := client.UpdateProductAPIKeyWithResponse(
			ctx, productCtx.ProductID, missingKey, ct.UpdateProductAPIKeyJSONRequestBody{Name: "Missing"},
		)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "PRODUCT_API_KEY_NOT_FOUND")
	})
	t.Run("Delete", func(t *testing.T) {
		resp, err := client.DeleteProductAPIKeyWithResponse(ctx, productCtx.ProductID, missingKey)
		require.NoError(t, err)
		requireNotFoundCode(t, resp.StatusCode(), resp.Body, "PRODUCT_API_KEY_NOT_FOUND")
	})
}

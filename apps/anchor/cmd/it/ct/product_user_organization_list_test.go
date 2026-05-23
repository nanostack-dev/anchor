package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListUserOrganizations(t *testing.T) {
	ctx := context.Background()

	t.Run("EmptyListWhenUserHasNoMemberships", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productUser := createDSLProductUser(t, productCtx)
		apiKeyClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:read"})

		resp, err := apiKeyClient.ListUserOrganizationsWithResponse(
			ctx,
			productCtx.ProductID,
			productUser.ID,
			nil,
		)

		require.NoError(t, err, "list user organizations request should not error")
		assert.Equal(t, http.StatusOK, resp.StatusCode(), "should return 200 OK")
		if assert.NotNil(t, resp.JSON200, "response body should not be nil") {
			assert.Empty(t, resp.JSON200.Items, "items should be empty when user has no memberships")
		}
	})

	t.Run("ReturnsOrganizationsWithRoleDetails", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productUser := createDSLProductUser(t, productCtx)
		apiKeyClient, _ := productCtx.CreateAPIKeyClientWithAllScopes()

		// Create an organization
		orgResp, err := apiKeyClient.CreateProductOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{
				Name:        "Test Org for Membership",
				Description: ptr.Ptr("Organization for membership testing"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, orgResp.StatusCode())
		require.NotNil(t, orgResp.JSON201)

		// Create a role
		role := createDSLProductRole(t, productCtx, "Member Role", ptr.Ptr("A member role"))
		createDSLMembership(t, productCtx, productUser.ID, orgResp.JSON201.Id, role.ID)

		// List user organizations
		readClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
		listResp, err := readClient.ListUserOrganizationsWithResponse(
			ctx,
			productCtx.ProductID,
			productUser.ID,
			nil,
		)

		require.NoError(t, err, "list user organizations request should not error")
		assert.Equal(t, http.StatusOK, listResp.StatusCode(), "should return 200 OK")
		if assert.NotNil(t, listResp.JSON200, "response body should not be nil") {
			require.Len(t, listResp.JSON200.Items, 1, "should have exactly one organization")
			item := listResp.JSON200.Items[0]
			assert.Equal(t, orgResp.JSON201.Id, item.Organization.Id, "organization ID should match")
			assert.Equal(t, "Test Org for Membership", item.Organization.Name, "organization name should match")
			assert.Equal(
				t, "Organization for membership testing",
				*item.Organization.Description, "organization description should match",
			)
			assert.Equal(t, role.ID, item.Role.Id, "role ID should match")
			assert.Equal(t, "Member Role", item.Role.Name, "role name should match")
			assert.Nil(t, item.Role.Permissions, "permissions should be nil when not included")
			assert.NotZero(t, item.JoinedAt, "joined_at should not be zero")
		}
	})

	t.Run("ReturnsMultipleOrganizations", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productUser := createDSLProductUser(t, productCtx)
		apiKeyClient, _ := productCtx.CreateAPIKeyClientWithAllScopes()

		// Create two organizations
		org1Resp, err := apiKeyClient.CreateProductOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{
				Name:        "First Organization",
				Description: ptr.Ptr("First org"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, org1Resp.StatusCode())

		org2Resp, err := apiKeyClient.CreateProductOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{
				Name:        "Second Organization",
				Description: ptr.Ptr("Second org"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, org2Resp.StatusCode())

		// Create roles
		role1 := createDSLProductRole(t, productCtx, "Admin", ptr.Ptr("Admin role"))
		role2 := createDSLProductRole(t, productCtx, "Viewer", ptr.Ptr("Viewer role"))
		createDSLMembership(t, productCtx, productUser.ID, org1Resp.JSON201.Id, role1.ID)
		createDSLMembership(t, productCtx, productUser.ID, org2Resp.JSON201.Id, role2.ID)

		readClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
		listResp, err := readClient.ListUserOrganizationsWithResponse(
			ctx,
			productCtx.ProductID,
			productUser.ID,
			nil,
		)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, listResp.StatusCode())
		if assert.NotNil(t, listResp.JSON200) {
			assert.Len(t, listResp.JSON200.Items, 2, "should have two organizations")
		}
	})

	t.Run("IncludeRolePermissions", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productCtx.CreateDefaultProductResourcePermissions(t)
		productUser := createDSLProductUser(t, productCtx)
		apiKeyClient, _ := productCtx.CreateAPIKeyClientWithAllScopes()

		// Create an organization
		orgResp, err := apiKeyClient.CreateProductOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{
				Name:        "Permissions Test Org",
				Description: ptr.Ptr("Org for permissions testing"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, orgResp.StatusCode())

		// Create role with permissions
		role := createDSLProductRoleWithPermissions(
			t,
			productCtx,
			"Editor",
			ptr.Ptr("Editor role with permissions"),
			[]string{"file:read", "file:update"},
		)
		createDSLMembership(t, productCtx, productUser.ID, orgResp.JSON201.Id, role.ID)

		// List with include=role_permissions
		readClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
		include := []ct.UserOrganizationInclude{ct.UserOrganizationIncludeRolePermissions}
		listResp, err := readClient.ListUserOrganizationsWithResponse(
			ctx,
			productCtx.ProductID,
			productUser.ID,
			&ct.ListUserOrganizationsParams{
				Include: &include,
			},
		)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, listResp.StatusCode())
		if assert.NotNil(t, listResp.JSON200) {
			require.Len(t, listResp.JSON200.Items, 1)
			item := listResp.JSON200.Items[0]
			assert.NotNil(t, item.Role.Permissions, "permissions should be included")
			if item.Role.Permissions != nil {
				assert.ElementsMatch(
					t,
					[]string{"file:read", "file:update"},
					*item.Role.Permissions,
					"permissions should match assigned role permissions",
				)
			}
		}
	})

	t.Run("NonExistentUserReturns404", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		apiKeyClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:read"})

		nonExistentUserID := ids.MustNew("puser")
		resp, err := apiKeyClient.ListUserOrganizationsWithResponse(
			ctx,
			productCtx.ProductID,
			nonExistentUserID,
			nil,
		)

		require.NoError(t, err, "request should not error")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), "should return 404 for non-existent user")
	})

	t.Run("InsufficientPermissionsReturns403", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productUser := createDSLProductUser(t, productCtx)

		// Create client with wrong scope
		apiKeyClient, apiKeyID := productCtx.CreateAPIKeyClientWithScopes(
			[]string{"organization:read"},
		)

		resp, err := apiKeyClient.ListUserOrganizationsWithResponse(
			ctx,
			productCtx.ProductID,
			productUser.ID,
			nil,
		)

		require.NoError(t, err, "request should not error")
		itshared.AssertProductAPIKeyInsufficientPermissions(
			t,
			resp,
			apiKeyID,
			[]string{"product_user:read"},
			[]string{"organization:read"},
		)
	})
}

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

func TestGetUserOrganization(t *testing.T) {
	ctx := context.Background()

	t.Run("SuccessfulGetUserOrganization", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productUser := createDSLProductUser(t, productCtx)
		apiKeyClient, _ := productCtx.CreateAPIKeyClientWithAllScopes()

		// Create an organization
		orgResp, err := apiKeyClient.CreateProductOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{
				Name:        "Get Test Organization",
				Description: ptr.Ptr("Organization for get test"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, orgResp.StatusCode())
		require.NotNil(t, orgResp.JSON201)

		// Create a role
		role := createDSLProductRole(t, productCtx, "Developer", ptr.Ptr("Developer role"))
		createDSLMembership(t, productCtx, productUser.ID, orgResp.JSON201.Id, role.ID)

		// Get the specific user organization
		readClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
		getResp, err := readClient.GetUserOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			productUser.ID,
			orgResp.JSON201.Id,
			nil,
		)

		require.NoError(t, err, "get user organization request should not error")
		assert.Equal(t, http.StatusOK, getResp.StatusCode(), "should return 200 OK")
		if assert.NotNil(t, getResp.JSON200, "response body should not be nil") {
			assert.Equal(t, orgResp.JSON201.Id, getResp.JSON200.Organization.Id, "organization ID should match")
			assert.Equal(
				t, "Get Test Organization", getResp.JSON200.Organization.Name, "organization name should match",
			)
			assert.Equal(
				t, "Organization for get test",
				*getResp.JSON200.Organization.Description, "description should match",
			)
			assert.Equal(t, role.ID, getResp.JSON200.Role.Id, "role ID should match")
			assert.Equal(t, "Developer", getResp.JSON200.Role.Name, "role name should match")
			assert.Nil(t, getResp.JSON200.Role.Permissions, "permissions should be nil when not included")
			assert.NotZero(t, getResp.JSON200.JoinedAt, "joined_at should not be zero")
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
				Name:        "Permissions Get Test Org",
				Description: ptr.Ptr("Org for permissions get testing"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, orgResp.StatusCode())

		// Create role with permissions
		role := createDSLProductRoleWithPermissions(
			t,
			productCtx,
			"Manager",
			ptr.Ptr("Manager role"),
			[]string{"file:read", "file:create", "file:delete"},
		)
		createDSLMembership(t, productCtx, productUser.ID, orgResp.JSON201.Id, role.ID)

		// Get with include=role_permissions
		readClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
		include := []ct.UserOrganizationInclude{ct.UserOrganizationIncludeRolePermissions}
		getResp, err := readClient.GetUserOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			productUser.ID,
			orgResp.JSON201.Id,
			&ct.GetUserOrganizationParams{
				Include: &include,
			},
		)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, getResp.StatusCode())
		if assert.NotNil(t, getResp.JSON200) {
			assert.NotNil(t, getResp.JSON200.Role.Permissions, "permissions should be included")
			if getResp.JSON200.Role.Permissions != nil {
				assert.ElementsMatch(
					t,
					[]string{"file:read", "file:create", "file:delete"},
					*getResp.JSON200.Role.Permissions,
					"permissions should match assigned role permissions",
				)
			}
		}
	})

	t.Run("RoleWithNoPermissionsReturnsEmptyArray", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productUser := createDSLProductUser(t, productCtx)
		apiKeyClient, _ := productCtx.CreateAPIKeyClientWithAllScopes()

		// Create an organization
		orgResp, err := apiKeyClient.CreateProductOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{
				Name:        "Empty Permissions Org",
				Description: ptr.Ptr("Org for empty permissions test"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, orgResp.StatusCode())

		// Create role without permissions
		role := createDSLProductRole(t, productCtx, "Basic", ptr.Ptr("Basic role with no permissions"))
		createDSLMembership(t, productCtx, productUser.ID, orgResp.JSON201.Id, role.ID)

		// Get with include=role_permissions
		readClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
		include := []ct.UserOrganizationInclude{ct.UserOrganizationIncludeRolePermissions}
		getResp, err := readClient.GetUserOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			productUser.ID,
			orgResp.JSON201.Id,
			&ct.GetUserOrganizationParams{
				Include: &include,
			},
		)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, getResp.StatusCode())
		if assert.NotNil(t, getResp.JSON200) {
			assert.NotNil(t, getResp.JSON200.Role.Permissions, "permissions should be included even if empty")
			if getResp.JSON200.Role.Permissions != nil {
				assert.Empty(t, *getResp.JSON200.Role.Permissions, "permissions should be empty array")
			}
		}
	})

	t.Run("NonExistentMembershipReturns404", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productUser := createDSLProductUser(t, productCtx)
		apiKeyClient, _ := productCtx.CreateAPIKeyClientWithAllScopes()

		// Create an organization but don't add user as member
		orgResp, err := apiKeyClient.CreateProductOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{
				Name:        "No Membership Org",
				Description: ptr.Ptr("User is not a member"),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, orgResp.StatusCode())

		readClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
		getResp, err := readClient.GetUserOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			productUser.ID,
			orgResp.JSON201.Id,
			nil,
		)

		require.NoError(t, err, "request should not error")
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode(), "should return 404 for non-existent membership")
	})

	t.Run("NonExistentUserReturns404", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		apiKeyClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:read"})

		nonExistentUserID := ids.MustNew("puser")
		nonExistentOrgID := ids.MustNew("org")
		getResp, err := apiKeyClient.GetUserOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			nonExistentUserID,
			nonExistentOrgID,
			nil,
		)

		require.NoError(t, err, "request should not error")
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode(), "should return 404 for non-existent user")
	})

	t.Run("InsufficientPermissionsReturns403", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productUser := createDSLProductUser(t, productCtx)

		// Create client with wrong scope
		apiKeyClient, apiKeyID := productCtx.CreateAPIKeyClientWithScopes(
			[]string{"organization:read"},
		)

		resp, err := apiKeyClient.GetUserOrganizationWithResponse(
			ctx,
			productCtx.ProductID,
			productUser.ID,
			ids.MustNew("org"),
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

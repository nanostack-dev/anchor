package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrganizationMemberInclude(t *testing.T) {
	ctx := context.Background()

	t.Run("WithoutIncludeOmitsRolePermissions", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productCtx.CreateDefaultProductResourcePermissions(t)
		productUser := createDSLProductUser(t, productCtx)
		org := productCtx.CreateOrganization(t, "Member Include Off", nil)
		role := createDSLProductRoleWithPermissions(
			t, productCtx, "Reader", new("Reader role"), []string{"file:read"},
		)
		createDSLMembership(t, productCtx, productUser.ID, org.Id, role.ID)

		readClient, _ := productCtx.CreateAPIKeyClientWithScopes(
			[]string{"organization_member:read"},
		)
		resp, err := readClient.GetOrganizationMemberWithResponse(
			ctx,
			productCtx.ProductID,
			org.Id,
			productUser.ID,
			nil,
		)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, productUser.ID, resp.JSON200.ProductUserId)
		assert.Nil(t, resp.JSON200.Role.Permissions, "permissions must be absent without the include")
	})

	t.Run("IncludeRolePermissions", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productCtx.CreateDefaultProductResourcePermissions(t)
		productUser := createDSLProductUser(t, productCtx)
		org := productCtx.CreateOrganization(t, "Member Include On", nil)
		role := createDSLProductRoleWithPermissions(
			t,
			productCtx,
			"Manager",
			new("Manager role"),
			[]string{"file:read", "file:create", "file:delete"},
		)
		createDSLMembership(t, productCtx, productUser.ID, org.Id, role.ID)

		readClient, _ := productCtx.CreateAPIKeyClientWithScopes(
			[]string{"organization_member:read"},
		)
		include := ct.OrganizationMemberIncludeParameter{
			ct.OrganizationMemberIncludeRolePermissions,
		}
		resp, err := readClient.GetOrganizationMemberWithResponse(
			ctx,
			productCtx.ProductID,
			org.Id,
			productUser.ID,
			&ct.GetOrganizationMemberParams{Include: &include},
		)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.NotNil(t, resp.JSON200.Role.Permissions, "permissions must be present with the include")
		assert.ElementsMatch(
			t,
			[]string{"file:read", "file:create", "file:delete"},
			*resp.JSON200.Role.Permissions,
		)
	})

	t.Run("IncludeRolePermissionsOnRoleWithoutPermissions", func(t *testing.T) {
		productCtx := createTestProductContext(t)
		productUser := createDSLProductUser(t, productCtx)
		org := productCtx.CreateOrganization(t, "Member Include Empty", nil)
		role := createDSLProductRole(t, productCtx, "Basic", new("Basic role"))
		createDSLMembership(t, productCtx, productUser.ID, org.Id, role.ID)

		readClient, _ := productCtx.CreateAPIKeyClientWithScopes(
			[]string{"organization_member:read"},
		)
		include := ct.OrganizationMemberIncludeParameter{
			ct.OrganizationMemberIncludeRolePermissions,
		}
		resp, err := readClient.GetOrganizationMemberWithResponse(
			ctx,
			productCtx.ProductID,
			org.Id,
			productUser.ID,
			&ct.GetOrganizationMemberParams{Include: &include},
		)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.NotNil(t, resp.JSON200.Role.Permissions, "permissions must be present with the include")
		assert.Empty(t, *resp.JSON200.Role.Permissions)
	})
}

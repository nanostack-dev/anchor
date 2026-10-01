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

type notFoundCase struct {
	name string
	call func(t *testing.T) (int, []byte)
}

func runNotFoundCases(t *testing.T, code string, cases []notFoundCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := tc.call(t)
			requireNotFoundCode(t, status, body, code)
		})
	}
}

func TestNotFoundCodes_ProductRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	productCtx := createTestProductContext(t)
	client, _ := productCtx.CreateAPIKeyClientWithAllScopes()
	missingRole := ids.MustNew("product_role")

	runNotFoundCases(t, "ROLE_NOT_FOUND", []notFoundCase{
		{"Get", func(t *testing.T) (int, []byte) {
			resp, err := client.GetProductRoleWithResponse(ctx, productCtx.ProductID, missingRole)
			require.NoError(t, err)
			return resp.StatusCode(), resp.Body
		}},
		{"Update", func(t *testing.T) (int, []byte) {
			resp, err := client.UpdateProductRoleWithResponse(
				ctx, productCtx.ProductID, missingRole, ct.UpdateProductRoleJSONRequestBody{Name: "Missing"},
			)
			require.NoError(t, err)
			return resp.StatusCode(), resp.Body
		}},
		{"Delete", func(t *testing.T) (int, []byte) {
			resp, err := client.DeleteProductRoleWithResponse(ctx, productCtx.ProductID, missingRole)
			require.NoError(t, err)
			return resp.StatusCode(), resp.Body
		}},
		{"AssignPermission", func(t *testing.T) (int, []byte) {
			resp, err := client.AssignPermissionToProductRoleWithResponse(
				ctx, productCtx.ProductID, missingRole,
				ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: "missing:permission"},
			)
			require.NoError(t, err)
			return resp.StatusCode(), resp.Body
		}},
		{"UnassignPermission", func(t *testing.T) (int, []byte) {
			resp, err := client.UnassignPermissionFromProductRoleWithResponse(
				ctx, productCtx.ProductID, missingRole, "missing:permission",
			)
			require.NoError(t, err)
			return resp.StatusCode(), resp.Body
		}},
	})
}

func TestNotFoundCodes_OrganizationAPIKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	productCtx := createTestProductContext(t)
	client, _ := productCtx.CreateAPIKeyClientWithAllScopes()
	org := productCtx.CreateOrganization(t, "Missing Key Org", nil)
	missingKey := ids.MustNew("organization_apikey")
	missingOrg := ids.MustNew("org")

	t.Run("MissingKey", func(t *testing.T) {
		runNotFoundCases(t, "ORGANIZATION_API_KEY_NOT_FOUND", []notFoundCase{
			{"Get", func(t *testing.T) (int, []byte) {
				resp, err := client.GetOrganizationAPIKeyWithResponse(ctx, productCtx.ProductID, org.Id, missingKey)
				require.NoError(t, err)
				return resp.StatusCode(), resp.Body
			}},
			{"Update", func(t *testing.T) (int, []byte) {
				resp, err := client.UpdateOrganizationAPIKeyWithResponse(
					ctx, productCtx.ProductID, org.Id, missingKey,
					ct.UpdateOrganizationAPIKeyJSONRequestBody{Name: "Missing"},
				)
				require.NoError(t, err)
				return resp.StatusCode(), resp.Body
			}},
			{"Delete", func(t *testing.T) (int, []byte) {
				resp, err := client.DeleteOrganizationAPIKeyWithResponse(ctx, productCtx.ProductID, org.Id, missingKey)
				require.NoError(t, err)
				return resp.StatusCode(), resp.Body
			}},
		})
	})
	t.Run("MissingOrganization", func(t *testing.T) {
		runNotFoundCases(t, "ORGANIZATION_NOT_FOUND", []notFoundCase{
			{"Create", func(t *testing.T) (int, []byte) {
				resp, err := client.CreateOrganizationAPIKeyWithResponse(
					ctx, productCtx.ProductID, missingOrg,
					ct.CreateOrganizationAPIKeyJSONRequestBody{
						Name:        "Missing",
						Permissions: []string{"missing:permission"},
					},
				)
				require.NoError(t, err)
				return resp.StatusCode(), resp.Body
			}},
			{"Search", func(t *testing.T) (int, []byte) {
				resp, err := client.SearchOrganizationAPIKeysWithResponse(
					ctx, productCtx.ProductID, missingOrg, ct.SearchOrganizationAPIKeysJSONRequestBody{},
				)
				require.NoError(t, err)
				return resp.StatusCode(), resp.Body
			}},
			{"Get", func(t *testing.T) (int, []byte) {
				resp, err := client.GetOrganizationAPIKeyWithResponse(ctx, productCtx.ProductID, missingOrg, missingKey)
				require.NoError(t, err)
				return resp.StatusCode(), resp.Body
			}},
			{"Update", func(t *testing.T) (int, []byte) {
				resp, err := client.UpdateOrganizationAPIKeyWithResponse(
					ctx, productCtx.ProductID, missingOrg, missingKey,
					ct.UpdateOrganizationAPIKeyJSONRequestBody{Name: "Missing"},
				)
				require.NoError(t, err)
				return resp.StatusCode(), resp.Body
			}},
			{"Delete", func(t *testing.T) (int, []byte) {
				resp, err := client.DeleteOrganizationAPIKeyWithResponse(
					ctx, productCtx.ProductID, missingOrg, missingKey,
				)
				require.NoError(t, err)
				return resp.StatusCode(), resp.Body
			}},
			{"Validate", func(t *testing.T) (int, []byte) {
				resp, err := client.ValidateOrganizationAPIKeyWithResponse(
					ctx, productCtx.ProductID, missingOrg,
					ct.ValidateOrganizationAPIKeyJSONRequestBody{ApiKey: "missing", RequiredScopes: []string{}},
				)
				require.NoError(t, err)
				return resp.StatusCode(), resp.Body
			}},
		})
	})
}

func TestNotFoundCodes_ProductUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	productCtx := createTestProductContext(t)
	client, _ := productCtx.CreateAPIKeyClientWithAllScopes()
	org := productCtx.CreateOrganization(t, "Missing User Org", nil)
	missingUser := ids.MustNew("pusr")

	runNotFoundCases(t, "PRODUCT_USER_NOT_FOUND", []notFoundCase{
		{"Get", func(t *testing.T) (int, []byte) {
			resp, err := client.GetProductUserWithResponse(ctx, productCtx.ProductID, missingUser)
			require.NoError(t, err)
			return resp.StatusCode(), resp.Body
		}},
		{"ListOrganizations", func(t *testing.T) (int, []byte) {
			resp, err := client.ListUserOrganizationsWithResponse(ctx, productCtx.ProductID, missingUser, nil)
			require.NoError(t, err)
			return resp.StatusCode(), resp.Body
		}},
		{"GetOrganization", func(t *testing.T) (int, []byte) {
			resp, err := client.GetUserOrganizationWithResponse(ctx, productCtx.ProductID, missingUser, org.Id, nil)
			require.NoError(t, err)
			return resp.StatusCode(), resp.Body
		}},
	})
}

func TestNotFoundCodes_UserOrganization(t *testing.T) {
	t.Parallel()
	productCtx := createTestProductContext(t)
	client, _ := productCtx.CreateAPIKeyClientWithAllScopes()
	org := productCtx.CreateOrganization(t, "Missing Membership Org", nil)
	user := createDSLProductUser(t, productCtx)

	resp, err := client.GetUserOrganizationWithResponse(
		context.Background(), productCtx.ProductID, user.ID, org.Id, nil,
	)

	require.NoError(t, err)
	requireNotFoundCode(t, resp.StatusCode(), resp.Body, "ORGANIZATION_MEMBERSHIP_NOT_FOUND")
}

func TestNotFoundCodes_Product(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := testOwnerClient(t)
	missingProduct := ids.MustNew("prd")

	runNotFoundCases(t, "PRODUCT_NOT_FOUND", []notFoundCase{
		{"Get", func(t *testing.T) (int, []byte) {
			resp, err := client.GetProductWithResponse(ctx, missingProduct)
			require.NoError(t, err)
			return resp.StatusCode(), resp.Body
		}},
		{"Update", func(t *testing.T) (int, []byte) {
			resp, err := client.UpdateProductWithResponse(
				ctx, missingProduct, ct.UpdateProductJSONRequestBody{Name: "Missing"},
			)
			require.NoError(t, err)
			return resp.StatusCode(), resp.Body
		}},
		{"Delete", func(t *testing.T) (int, []byte) {
			resp, err := client.DeleteProductWithResponse(ctx, missingProduct)
			require.NoError(t, err)
			return resp.StatusCode(), resp.Body
		}},
	})
}

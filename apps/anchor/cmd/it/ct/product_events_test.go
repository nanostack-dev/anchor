package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestProductEventsConfigAndDelivery(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	owner := product.OwnerAuthenticatedClient()
	sink := product.CaptureEvents()
	client, _ := product.CreateAPIKeyClientWithAllScopes()

	t.Run("GetReturnsObfuscatedSecretNotPlaintext", func(t *testing.T) {
		got, err := owner.GetProductWithResponse(ctx, product.ProductID)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, got.StatusCode())
		require.NotNil(t, got.JSON200.Config.Events)
		assert.Equal(t, sink.URL, got.JSON200.Config.Events.EndpointUrl)
		assert.NotEmpty(t, got.JSON200.Config.Events.SigningSecretObfuscated)
		assert.Nil(t, got.JSON200.Config.Events.SigningSecret)
	})

	t.Run("OrganizationCreatedUpdatedDeleted", func(t *testing.T) {
		created, err := client.CreateProductOrganizationWithResponse(
			ctx,
			product.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{Name: "Events Org " + ids.MustNew("org")},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode())
		orgID := created.JSON201.Id
		sink.WaitFor("organization.created", map[string]string{"organization_id": orgID})

		updated, updateErr := client.UpdateProductOrganizationWithResponse(
			ctx,
			product.ProductID,
			orgID,
			ct.UpdateProductOrganizationJSONRequestBody{Name: "Events Org Updated"},
		)
		require.NoError(t, updateErr)
		require.Equal(t, http.StatusOK, updated.StatusCode())
		sink.WaitFor("organization.updated", map[string]string{"organization_id": orgID})

		deleted, deleteErr := client.DeleteProductOrganizationWithResponse(ctx, product.ProductID, orgID)
		require.NoError(t, deleteErr)
		require.Equal(t, http.StatusNoContent, deleted.StatusCode())
		sink.WaitFor("organization.deleted", map[string]string{"organization_id": orgID})
	})

	t.Run("MembershipCreatedUpdatedDeleted", func(t *testing.T) {
		org, err := client.CreateProductOrganizationWithResponse(
			ctx,
			product.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{Name: "Events Members " + ids.MustNew("org")},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, org.StatusCode())
		role := createDSLProductRole(t, product, "Events Member Role", nil)
		otherRole := createDSLProductRole(t, product, "Events Other Role", nil)
		user := createDSLProductUser(t, product)

		added, addErr := client.AddOrganizationMemberWithResponse(
			ctx,
			product.ProductID,
			org.JSON201.Id,
			ct.AddOrganizationMemberJSONRequestBody{ProductUserId: user.ID, RoleId: role.ID},
		)
		require.NoError(t, addErr)
		require.Equal(t, http.StatusCreated, added.StatusCode())
		sink.WaitFor("organization.membership.created", map[string]string{
			"organization_id": org.JSON201.Id,
			"product_user_id": user.ID,
		})

		updated, updateErr := client.UpdateOrganizationMemberRoleWithResponse(
			ctx,
			product.ProductID,
			org.JSON201.Id,
			user.ID,
			ct.UpdateOrganizationMemberRoleJSONRequestBody{RoleId: otherRole.ID},
		)
		require.NoError(t, updateErr)
		require.Equal(t, http.StatusOK, updated.StatusCode())
		sink.WaitFor("organization.membership.updated", map[string]string{
			"organization_id": org.JSON201.Id,
			"product_user_id": user.ID,
		})

		removed, removeErr := client.RemoveOrganizationMemberWithResponse(
			ctx, product.ProductID, org.JSON201.Id, user.ID,
		)
		require.NoError(t, removeErr)
		require.Equal(t, http.StatusNoContent, removed.StatusCode())
		sink.WaitFor("organization.membership.deleted", map[string]string{
			"organization_id": org.JSON201.Id,
			"product_user_id": user.ID,
		})
	})

	t.Run("WorkspaceCreatedUpdatedDeleted", func(t *testing.T) {
		org := product.CreateOrganization(t, "Events Workspace Org "+ids.MustNew("org"), nil)
		created := createWorkspace(t, client, product.ProductID, org.Id, "events-ws-"+ids.MustNew("ws"))
		sink.WaitFor("workspace.created", map[string]string{
			"organization_id": org.Id,
			"workspace_id":    created.Id,
		})

		updated, err := client.UpdateOrganizationWorkspaceWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			created.Id,
			ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: created.Name + "-updated"},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, updated.StatusCode())
		sink.WaitFor("workspace.updated", map[string]string{
			"organization_id": org.Id,
			"workspace_id":    created.Id,
		})

		deleted, deleteErr := client.DeleteOrganizationWorkspaceWithResponse(
			ctx, product.ProductID, org.Id, created.Id,
		)
		require.NoError(t, deleteErr)
		require.Equal(t, http.StatusNoContent, deleted.StatusCode())
		sink.WaitFor("workspace.deleted", map[string]string{
			"organization_id": org.Id,
			"workspace_id":    created.Id,
		})
	})

	t.Run("OrganizationAPIKeyCreatedUpdatedDeleted", func(t *testing.T) {
		org := product.CreateOrganization(t, "Events API Key Org "+ids.MustNew("org"), nil)
		permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
		created, err := client.CreateOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			ct.CreateOrganizationAPIKeyJSONRequestBody{
				Name:        "events-key-" + ids.MustNew("key"),
				Permissions: []string{permissions.FileRead},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode())
		keyID := created.JSON201.Id
		sink.WaitFor("organization.api_key.created", map[string]string{
			"organization_id": org.Id,
			"api_key_id":      keyID,
		})

		updated, updateErr := client.UpdateOrganizationAPIKeyWithResponse(
			ctx,
			product.ProductID,
			org.Id,
			keyID,
			ct.UpdateOrganizationAPIKeyJSONRequestBody{Name: "events-key-updated"},
		)
		require.NoError(t, updateErr)
		require.Equal(t, http.StatusOK, updated.StatusCode())
		sink.WaitFor("organization.api_key.updated", map[string]string{
			"organization_id": org.Id,
			"api_key_id":      keyID,
		})

		deleted, deleteErr := client.DeleteOrganizationAPIKeyWithResponse(
			ctx, product.ProductID, org.Id, keyID,
		)
		require.NoError(t, deleteErr)
		require.Equal(t, http.StatusNoContent, deleted.StatusCode())
		sink.WaitFor("organization.api_key.deleted", map[string]string{
			"organization_id": org.Id,
			"api_key_id":      keyID,
		})
	})

	t.Run("ProductUserCreatedDeleted", func(t *testing.T) {
		email := itshared.Faker.Internet().Email()
		name := itshared.Faker.Person().Name()
		created, err := client.CreateProductUserWithResponse(
			ctx,
			product.ProductID,
			ct.CreateProductUserJSONRequestBody{Email: email, Name: &name},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode())
		userID := created.JSON201.Id
		sink.WaitFor("product_user.created", map[string]string{"product_user_id": userID})

		deleted, deleteErr := client.DeleteProductUserWithResponse(ctx, product.ProductID, userID)
		require.NoError(t, deleteErr)
		require.Equal(t, http.StatusNoContent, deleted.StatusCode())
		sink.WaitFor("product_user.deleted", map[string]string{"product_user_id": userID})
	})

	t.Run("RoleAndResourcePermissionCreatedUpdatedDeleted", func(t *testing.T) {
		createdPerm, err := owner.CreateProductResourcePermissionWithResponse(
			ctx,
			product.ProductID,
			ct.CreateProductResourcePermissionRequest{Name: "events:catalog"},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, createdPerm.StatusCode())
		permissionName := createdPerm.JSON201.Name
		sink.WaitFor("product.resource_permission.created", map[string]string{
			"permission_name": permissionName,
		})

		updatedPerm, updatePermErr := owner.UpdateProductResourcePermissionWithResponse(
			ctx,
			product.ProductID,
			permissionName,
			ct.UpdateProductResourcePermissionRequest{Description: new("catalog write")},
		)
		require.NoError(t, updatePermErr)
		require.Equal(t, http.StatusOK, updatedPerm.StatusCode())
		sink.WaitFor("product.resource_permission.updated", map[string]string{
			"permission_name": permissionName,
		})

		createdRole, roleErr := owner.CreateProductRoleWithResponse(
			ctx,
			product.ProductID,
			ct.CreateProductRoleJSONRequestBody{Name: "Events Role " + ids.MustNew("test")},
		)
		require.NoError(t, roleErr)
		require.Equal(t, http.StatusCreated, createdRole.StatusCode())
		roleID := createdRole.JSON201.Id
		sink.WaitFor("product.role.created", map[string]string{"role_id": roleID})

		assignResp, assignErr := owner.AssignPermissionToProductRoleWithResponse(
			ctx,
			product.ProductID,
			roleID,
			ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: permissionName},
		)
		require.NoError(t, assignErr)
		require.Equal(t, http.StatusNoContent, assignResp.StatusCode())
		require.Eventually(t, func() bool {
			return sink.Count("product.role.updated") == 1
		}, 20*time.Second, 200*time.Millisecond)

		unassignResp, unassignErr := owner.UnassignPermissionFromProductRoleWithResponse(
			ctx, product.ProductID, roleID, permissionName,
		)
		require.NoError(t, unassignErr)
		require.Equal(t, http.StatusNoContent, unassignResp.StatusCode())
		require.Eventually(t, func() bool {
			return sink.Count("product.role.updated") == 2
		}, 20*time.Second, 200*time.Millisecond)

		updatedRole, updateRoleErr := owner.UpdateProductRoleWithResponse(
			ctx,
			product.ProductID,
			roleID,
			ct.UpdateProductRoleJSONRequestBody{Name: "Events Role Updated " + ids.MustNew("test")},
		)
		require.NoError(t, updateRoleErr)
		require.Equal(t, http.StatusOK, updatedRole.StatusCode())
		require.Eventually(t, func() bool {
			return sink.Count("product.role.updated") == 3
		}, 20*time.Second, 200*time.Millisecond)

		deletedRole, deleteRoleErr := owner.DeleteProductRoleWithResponse(
			ctx, product.ProductID, roleID,
		)
		require.NoError(t, deleteRoleErr)
		require.Equal(t, http.StatusNoContent, deletedRole.StatusCode())
		sink.WaitFor("product.role.deleted", map[string]string{"role_id": roleID})

		deletedPerm, deletePermErr := owner.DeleteProductResourcePermissionWithResponse(
			ctx, product.ProductID, permissionName,
		)
		require.NoError(t, deletePermErr)
		require.Equal(t, http.StatusNoContent, deletedPerm.StatusCode())
		sink.WaitFor("product.resource_permission.deleted", map[string]string{
			"permission_name": permissionName,
		})
	})

	t.Run("GetProductEventsCatalog", func(t *testing.T) {
		catalogResp, err := owner.GetProductEventsCatalogWithResponse(ctx, product.ProductID)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, catalogResp.StatusCode())
		require.NotNil(t, catalogResp.JSON200)
		require.NotEmpty(t, catalogResp.JSON200.Items)

		themes := make(map[string]bool)
		integrations := make(map[string]bool)
		for _, item := range catalogResp.JSON200.Items {
			if item.GroupType == ct.Theme && item.Theme != nil {
				themes[*item.Theme] = true
			}
			if item.GroupType == ct.Integration && item.Integration != nil {
				integrations[*item.Integration] = true
			}
		}

		assert.True(t, themes["Organizations"], "Organizations theme must be present in catalog")
		assert.True(t, themes["Workspaces"], "Workspaces theme must be present in catalog")
		assert.True(t, themes["API Keys"], "API Keys theme must be present in catalog")
		assert.True(t, themes["Users"], "Users theme must be present in catalog")
		assert.True(t, themes["Licensing"], "Licensing theme must be present in catalog")
		assert.True(t, themes["Roles & Permissions"], "Roles & Permissions theme must be present in catalog")
		assert.True(t, integrations["CLERK"], "CLERK integration must be present in catalog")
		assert.False(t, integrations["SMTP"], "SMTP must not be present in catalog because it does not provide webhooks")
	})

	t.Run("EventSubscriptionFiltering", func(t *testing.T) {
		filterProduct := createTestProductContext(t)
		filterClient, _ := filterProduct.CreateAPIKeyClientWithAllScopes()
		filterOwner := filterProduct.OwnerAuthenticatedClient()

		// Subscribe only to organization.created
		filterSink := filterProduct.CaptureFilteredEvents([]string{"organization.created"})

		createdOrg, err := filterClient.CreateProductOrganizationWithResponse(
			ctx,
			filterProduct.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{Name: "Filter Org " + ids.MustNew("org")},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, createdOrg.StatusCode())
		orgID := createdOrg.JSON201.Id

		// organization.created must be delivered
		filterSink.WaitFor("organization.created", map[string]string{"organization_id": orgID})

		// Update org -> organization.updated was emitted, but should NOT be delivered to this sink
		updatedOrg, updateErr := filterClient.UpdateProductOrganizationWithResponse(
			ctx,
			filterProduct.ProductID,
			orgID,
			ct.UpdateProductOrganizationJSONRequestBody{Name: "Filter Org Renamed"},
		)
		require.NoError(t, updateErr)
		require.Equal(t, http.StatusOK, updatedOrg.StatusCode())

		// Create workspace -> workspace.created was emitted, but should NOT be delivered
		createdWs, wsErr := filterClient.CreateOrganizationWorkspaceWithResponse(
			ctx,
			filterProduct.ProductID,
			orgID,
			ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Filter Workspace"},
		)
		require.NoError(t, wsErr)
		require.Equal(t, http.StatusCreated, createdWs.StatusCode())

		// Give queue a moment to process jobs and assert un-subscribed events were never delivered
		time.Sleep(1 * time.Second)
		assert.Equal(t, 0, filterSink.Count("organization.updated"))
		assert.Equal(t, 0, filterSink.Count("workspace.created"))

		// Update product event subscription to now include organization.updated
		gotProduct, getErr := filterOwner.GetProductWithResponse(ctx, filterProduct.ProductID)
		require.NoError(t, getErr)
		_, updateProdErr := filterOwner.UpdateProductWithResponse(
			ctx,
			filterProduct.ProductID,
			ct.UpdateProductJSONRequestBody{
				Name:        gotProduct.JSON200.Name,
				Description: gotProduct.JSON200.Description,
				Config: &ct.ProductConfigRequest{
					OrganizationApiKeys: &ct.ProductOrganizationAPIKeysConfigRequest{
						Prefix: gotProduct.JSON200.Config.OrganizationApiKeys.Prefix,
					},
					Events: &ct.ProductEventsConfigRequest{
						EndpointUrl: &filterSink.URL,
						Events:      &[]string{"organization.created", "organization.updated"},
					},
				},
			},
		)
		require.NoError(t, updateProdErr)

		// Trigger organization.updated again -> now it must be delivered!
		_, updateAgainErr := filterClient.UpdateProductOrganizationWithResponse(
			ctx,
			filterProduct.ProductID,
			orgID,
			ct.UpdateProductOrganizationJSONRequestBody{Name: "Filter Org Renamed Again"},
		)
		require.NoError(t, updateAgainErr)
		filterSink.WaitFor("organization.updated", map[string]string{"organization_id": orgID})
	})
}

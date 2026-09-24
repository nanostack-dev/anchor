package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/pgkit/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
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

		internalGroups := make(map[string]bool)
		integrations := make(map[string]bool)
		for _, item := range catalogResp.JSON200.Items {
			if item.GroupType == ct.Internal {
				internalGroups[item.GroupName] = true
			}
			if item.GroupType == ct.Integration {
				integrations[item.GroupName] = true
			}
		}

		assert.True(t, internalGroups["Organizations"], "Organizations internal group must be in catalog")
		assert.True(t, internalGroups["Workspaces"], "Workspaces internal group must be in catalog")
		assert.True(t, internalGroups["API Keys"], "API Keys internal group must be in catalog")
		assert.True(t, internalGroups["Users"], "Users internal group must be in catalog")
		assert.True(t, internalGroups["Licensing"], "Licensing internal group must be in catalog")
		assert.True(t, internalGroups["Roles & Permissions"], "Roles internal group must be in catalog")
		assert.True(t, integrations["CLERK"], "CLERK integration must be in catalog")
		assert.False(
			t,
			integrations["SMTP"],
			"SMTP must not be present in catalog because it does not provide webhooks",
		)
	})

	t.Run("UnknownSubscriptionIsRejected", func(t *testing.T) {
		before, err := owner.GetProductWithResponse(ctx, product.ProductID)
		require.NoError(t, err)
		require.NotNil(t, before.JSON200)
		require.NotNil(t, before.JSON200.Config.Events)

		response, err := owner.UpdateProductWithResponse(
			ctx,
			product.ProductID,
			ct.UpdateProductJSONRequestBody{
				Name: before.JSON200.Name,
				Config: &ct.ProductConfigRequest{
					OrganizationApiKeys: &ct.ProductOrganizationAPIKeysConfigRequest{
						Prefix: before.JSON200.Config.OrganizationApiKeys.Prefix,
					},
					Events: &ct.ProductEventsConfigRequest{
						EndpointUrl: &sink.URL,
						Events:      &[]string{"typo.event"},
					},
				},
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode())

		after, err := owner.GetProductWithResponse(ctx, product.ProductID)
		require.NoError(t, err)
		require.NotNil(t, after.JSON200)
		require.NotNil(t, after.JSON200.Config.Events)
		assert.Equal(t, before.JSON200.Config.Events.Events, after.JSON200.Config.Events.Events)
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

		// Wait until both filtered jobs finish before asserting their absence.
		require.Eventually(t, func() bool {
			jobs, listErr := EventQueue.ListJobs(ctx, queue.ListJobsParams{
				QueueName: "product-events", Search: filterProduct.ProductID, Limit: 100,
			})
			if listErr != nil {
				return false
			}
			done := map[string]bool{}
			for _, job := range jobs {
				var payload struct {
					ProductID string `json:"product_id"`
					Type      string `json:"type"`
				}
				if json.Unmarshal(job.Payload, &payload) == nil && payload.ProductID == filterProduct.ProductID {
					done[payload.Type] = job.Status == queue.StatusDone
				}
			}
			return done["organization.updated"] && done["workspace.created"]
		}, 20*time.Second, 50*time.Millisecond)
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

func TestProductEventDeliveryStatus(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	owner := product.OwnerAuthenticatedClient()
	client, _ := product.CreateAPIKeyClientWithAllScopes()

	initial, err := owner.GetProductEventDeliveryStatusWithResponse(ctx, product.ProductID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, initial.StatusCode())
	require.Equal(t, 0, initial.JSON200.FailedCount)
	require.Equal(t, 0, initial.JSON200.RetryingCount)
	require.Nil(t, initial.JSON200.LastFailure)

	other := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "tenant.other", Isolated: true}).
		Product(itdsl.ProductOpts{Alias: "product.other", TenantAlias: "tenant.other"}).
		Build().Product("product.other")
	outsideTenant, err := owner.GetProductEventDeliveryStatusWithResponse(ctx, other.ProductID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, outsideTenant.StatusCode())

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	got, err := owner.GetProductWithResponse(ctx, product.ProductID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, got.StatusCode())
	eventTypes := []string{"organization.created"}
	configured, err := owner.UpdateProductWithResponse(ctx, product.ProductID, ct.UpdateProductJSONRequestBody{
		Name: got.JSON200.Name,
		Config: &ct.ProductConfigRequest{
			OrganizationApiKeys: &ct.ProductOrganizationAPIKeysConfigRequest{
				Prefix: got.JSON200.Config.OrganizationApiKeys.Prefix,
			},
			Events: &ct.ProductEventsConfigRequest{EndpointUrl: &server.URL, Events: &eventTypes},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, configured.StatusCode())

	created, err := client.CreateProductOrganizationWithResponse(
		ctx, product.ProductID, ct.CreateProductOrganizationJSONRequestBody{
			Name: "Failing Events Org " + ids.MustNew("org"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, created.StatusCode())

	require.Eventually(t, func() bool {
		status, getErr := owner.GetProductEventDeliveryStatusWithResponse(ctx, product.ProductID)
		return getErr == nil && status.JSON200 != nil && status.JSON200.FailedCount == 1
	}, 90*time.Second, 100*time.Millisecond)
	final, err := owner.GetProductEventDeliveryStatusWithResponse(ctx, product.ProductID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, final.StatusCode())
	require.Equal(t, 0, final.JSON200.RetryingCount)
	require.NotNil(t, final.JSON200.LastFailure)
	assert.Equal(t, "organization.created", final.JSON200.LastFailure.EventType)
	assert.Equal(t, 6, final.JSON200.LastFailure.Attempts)
	require.NotNil(t, final.JSON200.LastFailure.Error)
	assert.Contains(t, *final.JSON200.LastFailure.Error, "delivery status 503")
	assert.EqualValues(t, 6, attempts.Load())
}

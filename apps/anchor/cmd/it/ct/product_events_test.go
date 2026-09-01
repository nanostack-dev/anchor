package ct_test

import (
	"context"
	"net/http"
	"testing"

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
		created, err := client.CreateProductUserWithResponse(
			ctx,
			product.ProductID,
			ct.CreateProductUserJSONRequestBody{Email: email},
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
}



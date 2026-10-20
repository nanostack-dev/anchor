package ct_test

import (
	"context"
	"net/http"
	"testing"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accessWorld is one product with organization A (a member, a workspace, an API
// key) and organization B (a workspace the member cannot reach).
type accessWorld struct {
	product     *itdsl.ProductContext
	access      itdsl.AccessClient
	permissions organizationAPIKeyResourcePermissions
	orgA        ct.ProductOrganizationResponse
	orgB        ct.ProductOrganizationResponse
	workspaceA  string
	workspaceB  string
	role        *itdsl.ProductRole
	externalID  string
	userID      string
	apiKey      string
	apiKeyID    string
}

func newAccessWorld(t *testing.T) accessWorld {
	t.Helper()
	product := createTestProductContext(t)
	permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
	world := accessWorld{
		product:     product,
		access:      product.Access(),
		permissions: permissions,
		orgA:        product.CreateOrganization(t, "Org-A-"+uuid.NewString(), nil),
		orgB:        product.CreateOrganization(t, "Org-B-"+uuid.NewString(), nil),
	}
	world.workspaceA = product.Workspaces(world.orgA.Id).
		Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "A-" + uuid.NewString()}).
		Id
	world.workspaceB = product.Workspaces(world.orgB.Id).
		Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "B-" + uuid.NewString()}).
		Id
	world.role = createDSLProductRoleWithPermissions(
		t,
		product,
		"reader-"+uuid.NewString(),
		nil,
		[]string{permissions.FileRead},
	)
	world.userID, world.externalID = world.member(t, world.orgA.Id, world.role.ID)
	world.apiKeyID, world.apiKey = world.organizationAPIKey(t, world.orgA.Id, permissions.FileRead)
	return world
}

// member creates a product user with an external ID and a membership.
func (w accessWorld) member(t *testing.T, organizationID, roleID string) (string, string) {
	t.Helper()
	user := createDSLProductUser(t, w.product)
	externalID := "user_" + itshared.Faker.UUID().V4()
	setDSLProductUserExternalID(t, w.product, user.ID, externalID)
	createDSLMembership(t, w.product, user.ID, organizationID, roleID)
	return user.ID, externalID
}

func (w accessWorld) organizationAPIKey(t *testing.T, organizationID string, permissions ...string) (string, string) {
	t.Helper()
	resp, err := w.product.AllScopeAPIKeyClient().CreateOrganizationAPIKeyWithResponse(
		context.Background(), w.product.ProductID, organizationID,
		ct.CreateOrganizationAPIKeyJSONRequestBody{Name: "Key-" + uuid.NewString(), Permissions: permissions},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return resp.JSON201.Id, resp.JSON201.Value
}

func userOnWorkspace(
	externalID, organizationID, workspaceID string,
	actions ...string,
) ct.EvaluateAccessBatchJSONRequestBody {
	return ct.EvaluateAccessBatchJSONRequestBody{
		Subject: ct.AccessSubject{Type: ct.User, Id: externalID},
		Resource: ct.AccessResource{
			Type:       ct.Workspace,
			Id:         &workspaceID,
			Properties: &ct.AccessResourceProperties{OrganizationId: &organizationID},
		},
		Evaluations: accessItems(actions...),
	}
}

func accessItems(actions ...string) []ct.AccessEvaluationItem {
	items := make([]ct.AccessEvaluationItem, 0, len(actions))
	for _, action := range actions {
		items = append(items, ct.AccessEvaluationItem{Action: ct.AccessAction{Name: action}})
	}
	return items
}

func requireDenied(t *testing.T, response ct.AccessEvaluationsResponse, reason ct.AccessDenyReason) {
	t.Helper()
	assert.False(t, response.Decision)
	require.NotNil(t, response.Context.Reason)
	assert.Equal(t, reason, *response.Context.Reason)
}

func TestAccessEvaluation(t *testing.T) {
	t.Parallel()
	w := newAccessWorld(t)
	read := w.permissions.FileRead

	t.Run("A member with the permission is allowed on a workspace of the organization", func(t *testing.T) {
		response := w.access.EvaluateBatch(userOnWorkspace(w.externalID, w.orgA.Id, w.workspaceA, read))

		assert.True(t, response.Decision)
		require.Len(t, response.Evaluations, 1)
		assert.True(t, response.Evaluations[0].Decision)
		require.NotNil(t, response.Context.Organization)
		assert.Equal(t, w.orgA.Id, response.Context.Organization.Id)
		assert.Equal(t, w.orgA.Name, *response.Context.Organization.Name)
		require.NotNil(t, response.Context.Role)
		assert.Equal(t, w.role.ID, response.Context.Role.Id)
		require.NotNil(t, response.Context.ProductUser)
		assert.Equal(t, w.userID, response.Context.ProductUser.Id)
		assert.Equal(t, []string{read}, *response.Context.Permissions)
		assert.Nil(t, response.Context.Reason)
	})

	t.Run("An empty evaluation list asks only whether the member reaches the organization", func(t *testing.T) {
		response := w.access.EvaluateBatch(ct.EvaluateAccessBatchJSONRequestBody{
			Subject:     ct.AccessSubject{Type: ct.User, Id: w.externalID},
			Resource:    ct.AccessResource{Type: ct.Organization, Id: &w.orgA.Id},
			Evaluations: []ct.AccessEvaluationItem{},
		})

		assert.True(t, response.Decision)
		assert.Empty(t, response.Evaluations)
		assert.Equal(t, w.orgA.Id, response.Context.Organization.Id)
	})

	t.Run("A missing permission is denied and named", func(t *testing.T) {
		response := w.access.EvaluateBatch(
			userOnWorkspace(w.externalID, w.orgA.Id, w.workspaceA, read, w.permissions.FileDelete),
		)

		requireDenied(t, response, ct.MISSINGPERMISSION)
		assert.Equal(t, []string{w.permissions.FileDelete}, *response.Context.MissingPermissions)
		require.Len(t, response.Evaluations, 2)
		assert.True(t, response.Evaluations[0].Decision)
		assert.False(t, response.Evaluations[1].Decision)
		assert.Equal(t, ct.MISSINGPERMISSION, *response.Evaluations[1].Context.Reason)
	})

	t.Run("Deny on first deny leaves the remaining actions not evaluated", func(t *testing.T) {
		body := userOnWorkspace(w.externalID, w.orgA.Id, w.workspaceA, w.permissions.FileDelete, read)
		semantic := ct.DenyOnFirstDeny
		body.Options = &ct.AccessEvaluationsOptions{EvaluationsSemantic: &semantic}

		response := w.access.EvaluateBatch(body)

		requireDenied(t, response, ct.MISSINGPERMISSION)
		require.Len(t, response.Evaluations, 2)
		assert.Equal(t, ct.MISSINGPERMISSION, *response.Evaluations[0].Context.Reason)
		assert.Equal(t, ct.NOTEVALUATED, *response.Evaluations[1].Context.Reason)
	})

	t.Run("A workspace of another organization and an unknown workspace are refused the same way", func(t *testing.T) {
		for _, workspaceID := range []string{w.workspaceB, ids.MustNew("ws")} {
			response := w.access.EvaluateBatch(userOnWorkspace(w.externalID, w.orgA.Id, workspaceID, read))
			requireDenied(t, response, ct.WORKSPACENOTINORGANIZATION)
			assert.Nil(t, response.Context.Organization, "a refusal before the permission check carries no identity")
		}
	})

	t.Run("A user who is not a member of the organization is refused", func(t *testing.T) {
		response := w.access.EvaluateBatch(userOnWorkspace(w.externalID, w.orgB.Id, w.workspaceB, read))
		requireDenied(t, response, ct.NOTAMEMBER)
	})

	t.Run("An unknown organization is refused as not found", func(t *testing.T) {
		unknown := ids.MustNew("org")
		response := w.access.EvaluateBatch(ct.EvaluateAccessBatchJSONRequestBody{
			Subject:     ct.AccessSubject{Type: ct.User, Id: w.externalID},
			Resource:    ct.AccessResource{Type: ct.Organization, Id: &unknown},
			Evaluations: accessItems(read),
		})
		requireDenied(t, response, ct.ORGANIZATIONNOTFOUND)
	})

	t.Run("An external ID with no product user yet is refused as not synced", func(t *testing.T) {
		response := w.access.EvaluateBatch(userOnWorkspace("user_"+uuid.NewString(), w.orgA.Id, w.workspaceA, read))
		requireDenied(t, response, ct.SUBJECTNOTSYNCED)
	})

	t.Run("An organization API key is evaluated in its own organization", func(t *testing.T) {
		response := w.access.EvaluateBatch(ct.EvaluateAccessBatchJSONRequestBody{
			Subject:     ct.AccessSubject{Type: ct.OrganizationApiKey, Id: w.apiKey},
			Resource:    ct.AccessResource{Type: ct.Workspace, Id: &w.workspaceA},
			Evaluations: accessItems(read),
		})

		assert.True(t, response.Decision)
		assert.Equal(t, w.orgA.Id, response.Context.Organization.Id)
		require.NotNil(t, response.Context.ApiKey)
		assert.Equal(t, w.apiKeyID, response.Context.ApiKey.Id)
		assert.Nil(t, response.Context.ProductUser)
	})

	t.Run("An organization API key cannot reach another organization's workspace", func(t *testing.T) {
		response := w.access.EvaluateBatch(ct.EvaluateAccessBatchJSONRequestBody{
			Subject:     ct.AccessSubject{Type: ct.OrganizationApiKey, Id: w.apiKey},
			Resource:    ct.AccessResource{Type: ct.Workspace, Id: &w.workspaceB},
			Evaluations: accessItems(read),
		})
		requireDenied(t, response, ct.WORKSPACENOTINORGANIZATION)
	})

	t.Run("An organization API key naming another organization is not a member", func(t *testing.T) {
		response := w.access.EvaluateBatch(ct.EvaluateAccessBatchJSONRequestBody{
			Subject:     ct.AccessSubject{Type: ct.OrganizationApiKey, Id: w.apiKey},
			Resource:    ct.AccessResource{Type: ct.Organization, Id: &w.orgB.Id},
			Evaluations: accessItems(read),
		})
		requireDenied(t, response, ct.NOTAMEMBER)
	})

	t.Run("An unknown API key is refused as invalid", func(t *testing.T) {
		response := w.access.EvaluateBatch(ct.EvaluateAccessBatchJSONRequestBody{
			Subject:     ct.AccessSubject{Type: ct.OrganizationApiKey, Id: "not-a-key-" + uuid.NewString()},
			Resource:    ct.AccessResource{Type: ct.Organization},
			Evaluations: accessItems(read),
		})
		requireDenied(t, response, ct.APIKEYINVALID)
	})

	t.Run("The single evaluation endpoint decides one action", func(t *testing.T) {
		response := w.access.Evaluate(ct.EvaluateAccessJSONRequestBody{
			Subject:  ct.AccessSubject{Type: ct.User, Id: w.externalID},
			Resource: ct.AccessResource{Type: ct.Organization, Id: &w.orgA.Id},
			Action:   ct.AccessAction{Name: w.permissions.FileDelete},
		})

		assert.False(t, response.Decision)
		assert.Equal(t, ct.MISSINGPERMISSION, *response.Context.Reason)
		assert.Equal(t, []string{w.permissions.FileDelete}, *response.Context.MissingPermissions)
	})

	t.Run("The SDK decides the same as the API", func(t *testing.T) {
		decision, err := w.product.SDKClient().AccessForUser(w.externalID).
			InWorkspace(w.orgA.Id, w.workspaceA).
			Permissions(read).
			Do(t.Context())

		require.NoError(t, err)
		assert.True(t, decision.Decision)
		assert.Equal(t, w.orgA.Id, decision.Context.Organization.Id)
	})

	t.Run("A user subject without an organization is a bad request", func(t *testing.T) {
		resp := w.access.EvaluateBatchRaw(ct.EvaluateAccessBatchJSONRequestBody{
			Subject:     ct.AccessSubject{Type: ct.User, Id: w.externalID},
			Resource:    ct.AccessResource{Type: ct.Workspace, Id: &w.workspaceA},
			Evaluations: accessItems(read),
		})
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	})

	t.Run("A product key without the read scopes cannot evaluate", func(t *testing.T) {
		limited, _ := w.product.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
		resp := w.product.AccessClientWith(limited).
			EvaluateBatchRaw(userOnWorkspace(w.externalID, w.orgA.Id, w.workspaceA, read))
		assert.Equal(t, http.StatusForbidden, resp.StatusCode(), string(resp.Body))
	})
}

// Each subtest changes Anchor and evaluates again at once: nothing is cached, so
// the next evaluation sees the write.
func TestAccessEvaluationSeesWritesOnTheNextCall(t *testing.T) {
	t.Parallel()

	t.Run("A deleted workspace is refused on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		workspaces := w.product.Workspaces(w.orgA.Id)
		workspaceID := workspaces.Create(
			ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Temp-" + uuid.NewString()},
		).Id
		require.True(
			t,
			w.access.EvaluateBatch(
				userOnWorkspace(w.externalID, w.orgA.Id, workspaceID, w.permissions.FileRead),
			).Decision,
		)

		workspaces.Delete(workspaceID)

		requireDenied(
			t,
			w.access.EvaluateBatch(userOnWorkspace(w.externalID, w.orgA.Id, workspaceID, w.permissions.FileRead)),
			ct.WORKSPACENOTINORGANIZATION,
		)
	})

	t.Run("A removed member is refused on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		require.True(
			t,
			w.access.EvaluateBatch(
				userOnWorkspace(w.externalID, w.orgA.Id, w.workspaceA, w.permissions.FileRead),
			).Decision,
		)

		resp, err := w.product.AllScopeAPIKeyClient().RemoveOrganizationMemberWithResponse(
			context.Background(), w.product.ProductID, w.orgA.Id, w.userID)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

		requireDenied(
			t,
			w.access.EvaluateBatch(userOnWorkspace(w.externalID, w.orgA.Id, w.workspaceA, w.permissions.FileRead)),
			ct.NOTAMEMBER,
		)
	})

	t.Run("A permission added to the role applies on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		update := w.permissions.FileUpdate
		requireDenied(
			t,
			w.access.EvaluateBatch(userOnWorkspace(w.externalID, w.orgA.Id, w.workspaceA, update)),
			ct.MISSINGPERMISSION,
		)

		resp, err := w.product.AllScopeAPIKeyClient().AssignPermissionToProductRoleWithResponse(
			context.Background(),
			w.product.ProductID, w.role.ID, ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: update})
		require.NoError(t, err)
		require.Less(t, resp.StatusCode(), 300, string(resp.Body))

		assert.True(t, w.access.EvaluateBatch(userOnWorkspace(w.externalID, w.orgA.Id, w.workspaceA, update)).Decision)
	})

	t.Run("A deactivated organization API key is refused as inactive", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		inactive := ct.OrganizationAPIKeyStatusINACTIVE

		resp, err := w.product.AllScopeAPIKeyClient().UpdateOrganizationAPIKeyWithResponse(
			context.Background(), w.product.ProductID, w.orgA.Id, w.apiKeyID,
			ct.UpdateOrganizationAPIKeyJSONRequestBody{Name: "Key-" + uuid.NewString(), Status: &inactive})
		require.NoError(t, err)
		require.Less(t, resp.StatusCode(), 300, string(resp.Body))

		requireDenied(t, w.access.EvaluateBatch(ct.EvaluateAccessBatchJSONRequestBody{
			Subject:     ct.AccessSubject{Type: ct.OrganizationApiKey, Id: w.apiKey},
			Resource:    ct.AccessResource{Type: ct.Organization},
			Evaluations: accessItems(w.permissions.FileRead),
		}), ct.APIKEYINACTIVE)
	})

	t.Run("A deleted organization API key is refused on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		body := ct.EvaluateAccessBatchJSONRequestBody{
			Subject:     ct.AccessSubject{Type: ct.OrganizationApiKey, Id: w.apiKey},
			Resource:    ct.AccessResource{Type: ct.Organization},
			Evaluations: accessItems(w.permissions.FileRead),
		}
		require.True(t, w.access.EvaluateBatch(body).Decision)

		resp, err := w.product.AllScopeAPIKeyClient().DeleteOrganizationAPIKeyWithResponse(
			context.Background(), w.product.ProductID, w.orgA.Id, w.apiKeyID)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

		requireDenied(t, w.access.EvaluateBatch(body), ct.APIKEYINVALID)
	})
}

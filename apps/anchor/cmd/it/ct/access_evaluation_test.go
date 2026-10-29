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
	product    *itdsl.ProductContext
	access     itdsl.AccessClient
	read       string
	update     string
	remove     string
	orgA       ct.ProductOrganizationResponse
	orgB       ct.ProductOrganizationResponse
	workspaceA string
	workspaceB string
	role       *itdsl.ProductRole
	externalID string
	userID     string
	userEmail  string
	apiKey     string
	apiKeyID   string
	apiKeyName string
}

func newAccessWorld(t *testing.T) accessWorld {
	t.Helper()
	product := createTestProductContext(t)
	permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
	w := accessWorld{
		product: product,
		access:  product.Access(),
		read:    permissions.FileRead,
		update:  permissions.FileUpdate,
		remove:  permissions.FileDelete,
		orgA:    product.CreateOrganization(t, "Org-A-"+uuid.NewString(), nil),
		orgB:    product.CreateOrganization(t, "Org-B-"+uuid.NewString(), nil),
	}
	w.workspaceA = w.workspace(w.orgA.Id)
	w.workspaceB = w.workspace(w.orgB.Id)
	w.role = createDSLProductRoleWithPermissions(t, product, "reader-"+uuid.NewString(), nil, []string{w.read})
	var user *itdsl.ProductUser
	user, w.externalID = w.member(t, w.orgA.Id, w.role.ID)
	w.userID, w.userEmail = user.ID, user.Email
	w.apiKeyName = "Key-" + uuid.NewString()
	w.apiKeyID, w.apiKey = w.organizationAPIKey(t, w.orgA.Id, w.apiKeyName, w.read)
	return w
}

func (w accessWorld) workspace(organizationID string) string {
	return w.product.Workspaces(organizationID).
		Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Ws-" + uuid.NewString()}).Id
}

// member creates a product user with an external ID and a membership.
func (w accessWorld) member(t *testing.T, organizationID, roleID string) (*itdsl.ProductUser, string) {
	t.Helper()
	user := createDSLProductUser(t, w.product)
	externalID := "user_" + itshared.Faker.UUID().V4()
	setDSLProductUserExternalID(t, w.product, user.ID, externalID)
	createDSLMembership(t, w.product, user.ID, organizationID, roleID)
	return user, externalID
}

func (w accessWorld) organizationAPIKey(
	t *testing.T, organizationID, name string, permissions ...string,
) (string, string) {
	t.Helper()
	resp, err := w.product.AllScopeAPIKeyClient().CreateOrganizationAPIKeyWithResponse(
		context.Background(), w.product.ProductID, organizationID,
		ct.CreateOrganizationAPIKeyJSONRequestBody{Name: name, Permissions: permissions},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return resp.JSON201.Id, resp.JSON201.Value
}

// memberInWorkspaceA is the world's member asking about workspace A.
func (w accessWorld) memberInWorkspaceA() itdsl.AccessQuestion {
	return w.access.User(w.externalID).InWorkspace(w.orgA.Id, w.workspaceA)
}

func requireDenied(t *testing.T, response ct.AccessEvaluationsResponse, reason ct.AccessDenyReason) {
	t.Helper()
	assert.False(t, response.Decision)
	require.NotNil(t, response.Context.Reason)
	assert.Equal(t, reason, *response.Context.Reason)
}

func requireStatus(t *testing.T, want, got int, body []byte) {
	t.Helper()
	assert.Equal(t, want, got, string(body))
}

func TestAccessEvaluation(t *testing.T) {
	t.Parallel()
	w := newAccessWorld(t)

	t.Run("A member with the permission is allowed on a workspace of the organization", func(t *testing.T) {
		response := w.memberInWorkspaceA().Can(w.read).Decide()

		assert.True(t, response.Decision)
		require.Len(t, response.Evaluations, 1)
		assert.True(t, response.Evaluations[0].Decision)
		assert.Nil(t, response.Context.Reason)
		assert.Equal(t, w.orgA.Id, response.Context.Organization.Id)
		assert.Equal(t, w.orgA.Name, *response.Context.Organization.Name)
		assert.Equal(t, w.role.ID, response.Context.Role.Id)
		assert.Equal(t, w.userID, response.Context.ProductUser.Id)
		assert.Equal(t, w.userEmail, *response.Context.ProductUser.Email)
		assert.Equal(t, []string{w.read}, *response.Context.Permissions)
	})

	t.Run("An empty evaluation list asks only whether the member reaches the organization", func(t *testing.T) {
		response := w.access.User(w.externalID).InOrganization(w.orgA.Id).Decide()

		assert.True(t, response.Decision)
		assert.Empty(t, response.Evaluations)
		assert.Equal(t, w.orgA.Id, response.Context.Organization.Id)
	})

	t.Run("A missing permission is denied and named", func(t *testing.T) {
		response := w.memberInWorkspaceA().Can(w.read, w.remove).Decide()

		requireDenied(t, response, ct.MISSINGPERMISSION)
		assert.Equal(t, []string{w.remove}, *response.Context.MissingPermissions)
		require.Len(t, response.Evaluations, 2)
		assert.True(t, response.Evaluations[0].Decision)
		assert.Equal(t, ct.MISSINGPERMISSION, *response.Evaluations[1].Context.Reason)
	})

	t.Run("Deny on first deny leaves the remaining actions not evaluated", func(t *testing.T) {
		response := w.memberInWorkspaceA().Can(w.remove, w.read).DenyOnFirstDeny().Decide()

		requireDenied(t, response, ct.MISSINGPERMISSION)
		require.Len(t, response.Evaluations, 2)
		assert.Equal(t, ct.MISSINGPERMISSION, *response.Evaluations[0].Context.Reason)
		assert.Equal(t, ct.NOTEVALUATED, *response.Evaluations[1].Context.Reason)
	})

	t.Run("A workspace of another organization and an unknown workspace are refused the same way", func(t *testing.T) {
		for _, workspaceID := range []string{w.workspaceB, ids.MustNew("ws")} {
			response := w.access.User(w.externalID).InWorkspace(w.orgA.Id, workspaceID).Can(w.read).Decide()

			requireDenied(t, response, ct.WORKSPACENOTINORGANIZATION)
			assert.Nil(t, response.Context.Organization, "a refusal before the permission check carries no identity")
		}
	})

	t.Run("A user who is not a member of the organization is refused", func(t *testing.T) {
		response := w.access.User(w.externalID).InWorkspace(w.orgB.Id, w.workspaceB).Can(w.read).Decide()
		requireDenied(t, response, ct.NOTAMEMBER)
	})

	t.Run("An unknown organization is refused as not found", func(t *testing.T) {
		response := w.access.User(w.externalID).InOrganization(ids.MustNew("org")).Can(w.read).Decide()
		requireDenied(t, response, ct.ORGANIZATIONNOTFOUND)
	})

	t.Run("An external ID with no product user yet is refused as not synced", func(t *testing.T) {
		response := w.access.User("user_"+uuid.NewString()).InWorkspace(w.orgA.Id, w.workspaceA).Can(w.read).Decide()
		requireDenied(t, response, ct.SUBJECTNOTSYNCED)
	})

	t.Run("An organization API key is evaluated in its own organization", func(t *testing.T) {
		response := w.access.APIKey(w.apiKey).InWorkspace("", w.workspaceA).Can(w.read).Decide()

		assert.True(t, response.Decision)
		assert.Equal(t, w.orgA.Id, response.Context.Organization.Id)
		assert.Equal(t, w.apiKeyID, response.Context.ApiKey.Id)
		assert.Equal(t, w.apiKeyName, *response.Context.ApiKey.Name)
		assert.Nil(t, response.Context.ProductUser)
	})

	t.Run("An organization API key cannot reach another organization's workspace", func(t *testing.T) {
		response := w.access.APIKey(w.apiKey).InWorkspace("", w.workspaceB).Can(w.read).Decide()
		requireDenied(t, response, ct.WORKSPACENOTINORGANIZATION)
	})

	t.Run("An organization API key naming another organization is not a member", func(t *testing.T) {
		response := w.access.APIKey(w.apiKey).InOrganization(w.orgB.Id).Can(w.read).Decide()
		requireDenied(t, response, ct.NOTAMEMBER)
	})

	t.Run("An unknown API key is refused as invalid", func(t *testing.T) {
		response := w.access.APIKey("not-a-key-" + uuid.NewString()).Can(w.read).Decide()
		requireDenied(t, response, ct.APIKEYINVALID)
	})

	t.Run("The single evaluation endpoint decides one action", func(t *testing.T) {
		resp := w.access.User(w.externalID).InOrganization(w.orgA.Id).Can(w.remove).Single()

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.False(t, resp.JSON200.Decision)
		assert.Equal(t, ct.MISSINGPERMISSION, *resp.JSON200.Context.Reason)
		assert.Equal(t, []string{w.remove}, *resp.JSON200.Context.MissingPermissions)
	})

	t.Run("The SDK decides the same as the API", func(t *testing.T) {
		decision, err := w.product.SDKClient().AccessForUser(w.externalID).
			InWorkspace(w.orgA.Id, w.workspaceA).
			Permissions(w.read).
			Do(t.Context())

		require.NoError(t, err)
		assert.True(t, decision.Decision)
		assert.Equal(t, w.orgA.Id, decision.Context.Organization.Id)
	})

	t.Run("A user subject without an organization is a bad request", func(t *testing.T) {
		resp := w.access.User(w.externalID).InWorkspace("", w.workspaceA).Can(w.read).Send()
		requireStatus(t, http.StatusBadRequest, resp.StatusCode(), resp.Body)
	})

	t.Run("A product key without the read scopes cannot evaluate", func(t *testing.T) {
		limited, _ := w.product.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
		resp := w.product.AccessClientWith(limited).User(w.externalID).InOrganization(w.orgA.Id).Can(w.read).Send()
		requireStatus(t, http.StatusForbidden, resp.StatusCode(), resp.Body)
	})
}

// Each subtest changes Anchor and evaluates again at once: nothing is cached, so
// the next evaluation sees the write.
func TestAccessEvaluationSeesWritesOnTheNextCall(t *testing.T) {
	t.Parallel()
	admin := func(w accessWorld) *ct.ClientWithResponses { return w.product.AllScopeAPIKeyClient() }

	t.Run("A deleted workspace is refused on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		workspaceID := w.workspace(w.orgA.Id)
		question := w.access.User(w.externalID).InWorkspace(w.orgA.Id, workspaceID).Can(w.read)
		require.True(t, question.Decide().Decision)

		w.product.Workspaces(w.orgA.Id).Delete(workspaceID)

		requireDenied(t, question.Decide(), ct.WORKSPACENOTINORGANIZATION)
	})

	t.Run("A removed member is refused on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		question := w.memberInWorkspaceA().Can(w.read)
		require.True(t, question.Decide().Decision)

		resp, err := admin(
			w,
		).RemoveOrganizationMemberWithResponse(t.Context(), w.product.ProductID, w.orgA.Id, w.userID)
		require.NoError(t, err)
		requireStatus(t, http.StatusNoContent, resp.StatusCode(), resp.Body)

		requireDenied(t, question.Decide(), ct.NOTAMEMBER)
	})

	t.Run("A permission added to the role applies on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		question := w.memberInWorkspaceA().Can(w.update)
		requireDenied(t, question.Decide(), ct.MISSINGPERMISSION)

		resp, err := admin(w).AssignPermissionToProductRoleWithResponse(t.Context(), w.product.ProductID, w.role.ID,
			ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: w.update})
		require.NoError(t, err)
		require.Less(t, resp.StatusCode(), 300, string(resp.Body))

		assert.True(t, question.Decide().Decision)
	})

	t.Run("A permission removed from the role is refused on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		question := w.memberInWorkspaceA().Can(w.read)
		require.True(t, question.Decide().Decision)

		resp, err := admin(w).UnassignPermissionFromProductRoleWithResponse(
			t.Context(), w.product.ProductID, w.role.ID, w.read)
		require.NoError(t, err)
		require.Less(t, resp.StatusCode(), 300, string(resp.Body))

		requireDenied(t, question.Decide(), ct.MISSINGPERMISSION)
	})

	t.Run("A member moved to a role without the permission is refused on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		question := w.memberInWorkspaceA().Can(w.read)
		require.True(t, question.Decide().Decision)
		other := createDSLProductRoleWithPermissions(t, w.product, "other-"+uuid.NewString(), nil, []string{w.update})

		resp, err := admin(w).UpdateOrganizationMemberRoleWithResponse(t.Context(), w.product.ProductID, w.orgA.Id,
			w.userID, ct.UpdateOrganizationMemberRoleJSONRequestBody{RoleId: &other.ID})
		require.NoError(t, err)
		require.Less(t, resp.StatusCode(), 300, string(resp.Body))

		requireDenied(t, question.Decide(), ct.MISSINGPERMISSION)
		assert.True(t, w.memberInWorkspaceA().Can(w.update).Decide().Decision)
	})

	t.Run("A permission deleted from the catalog is refused for an API key on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		question := w.access.APIKey(w.apiKey).Can(w.read)
		require.True(t, question.Decide().Decision)

		resp, err := admin(w).DeleteProductResourcePermissionWithResponse(t.Context(), w.product.ProductID, w.read)
		require.NoError(t, err)
		require.Less(t, resp.StatusCode(), 300, string(resp.Body))

		requireDenied(t, question.Decide(), ct.MISSINGPERMISSION)
	})

	t.Run("A deactivated organization API key is refused as inactive", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		inactive := ct.OrganizationAPIKeyStatusINACTIVE

		resp, err := admin(
			w,
		).UpdateOrganizationAPIKeyWithResponse(t.Context(), w.product.ProductID, w.orgA.Id, w.apiKeyID,
			ct.UpdateOrganizationAPIKeyJSONRequestBody{Name: "Key-" + uuid.NewString(), Status: &inactive})
		require.NoError(t, err)
		require.Less(t, resp.StatusCode(), 300, string(resp.Body))

		requireDenied(t, w.access.APIKey(w.apiKey).Can(w.read).Decide(), ct.APIKEYINACTIVE)
	})

	t.Run("A deleted organization API key is refused on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		question := w.access.APIKey(w.apiKey).Can(w.read)
		require.True(t, question.Decide().Decision)

		resp, err := admin(
			w,
		).DeleteOrganizationAPIKeyWithResponse(t.Context(), w.product.ProductID, w.orgA.Id, w.apiKeyID)
		require.NoError(t, err)
		requireStatus(t, http.StatusNoContent, resp.StatusCode(), resp.Body)

		requireDenied(t, question.Decide(), ct.APIKEYINVALID)
	})
}

func TestAccessEvaluationBatchFollowsAuthZENDefaults(t *testing.T) {
	t.Parallel()
	w := newAccessWorld(t)

	t.Run("A top-level action with no items is evaluated alone", func(t *testing.T) {
		response := w.memberInWorkspaceA().Default(w.remove).Items().Decide()

		requireDenied(t, response, ct.MISSINGPERMISSION)
		assert.Equal(t, []string{w.remove}, *response.Context.MissingPermissions)
	})

	t.Run("A top-level action is the default for items without one", func(t *testing.T) {
		response := w.memberInWorkspaceA().Default(w.remove).
			Items(ct.AccessEvaluationItem{}, ct.AccessEvaluationItem{Action: &ct.AccessAction{Name: w.read}}).
			Decide()

		require.Len(t, response.Evaluations, 2)
		assert.False(t, response.Evaluations[0].Decision, "the default action is evaluated")
		assert.True(t, response.Evaluations[1].Decision)
	})

	t.Run("A per-item resource override is refused instead of ignored", func(t *testing.T) {
		resp := w.memberInWorkspaceA().Items(ct.AccessEvaluationItem{
			Action:   &ct.AccessAction{Name: w.read},
			Resource: &ct.AccessResource{Type: ct.Workspace, Id: &w.workspaceB},
		}).Send()

		requireStatus(t, http.StatusBadRequest, resp.StatusCode(), resp.Body)
	})

	t.Run("A per-item subject override is refused instead of ignored", func(t *testing.T) {
		resp := w.memberInWorkspaceA().Items(ct.AccessEvaluationItem{
			Action:  &ct.AccessAction{Name: w.read},
			Subject: &ct.AccessSubject{Type: ct.OrganizationApiKey, Id: w.apiKey},
		}).Send()

		requireStatus(t, http.StatusBadRequest, resp.StatusCode(), resp.Body)
	})
}

func TestAccessEvaluationRejectsMalformedRequests(t *testing.T) {
	t.Parallel()
	w := newAccessWorld(t)
	cases := map[string]itdsl.AccessQuestion{
		"A blank subject id is a bad request": w.access.User("  ").
			InWorkspace(w.orgA.Id, w.workspaceA).
			Can(w.read),
		"An empty action name is a bad request":      w.memberInWorkspaceA().Can(""),
		"A workspace without an id is a bad request": w.access.APIKey(w.apiKey).InWorkspace("", "").Can(w.read),
		"An item with no action and no default is a bad request": w.access.User(w.externalID).
			InOrganization(w.orgA.Id).Items(ct.AccessEvaluationItem{}),
		"An unknown subject type is a bad request": w.access.As("service_account", w.externalID).
			InOrganization(w.orgA.Id).Can(w.read),
	}
	for name, question := range cases {
		t.Run(name, func(t *testing.T) {
			resp := question.Send()
			requireStatus(t, http.StatusBadRequest, resp.StatusCode(), resp.Body)
		})
	}

	t.Run("The single evaluation refuses an empty action name", func(t *testing.T) {
		resp := w.access.User(w.externalID).InOrganization(w.orgA.Id).Can("").Single()
		requireStatus(t, http.StatusBadRequest, resp.StatusCode(), resp.Body)
	})
}

func TestAccessEvaluationForARoleWithoutPermissions(t *testing.T) {
	t.Parallel()
	t.Run("A member whose role holds nothing reaches the organization but no action", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		empty := createDSLProductRoleWithPermissions(t, w.product, "empty-"+uuid.NewString(), nil, []string{})
		_, externalID := w.member(t, w.orgA.Id, empty.ID)
		inWorkspace := w.access.User(externalID).InWorkspace(w.orgA.Id, w.workspaceA)

		reach := inWorkspace.Decide()
		action := inWorkspace.Can(w.read).Decide()

		assert.True(t, reach.Decision)
		assert.Empty(t, *reach.Context.Permissions)
		requireDenied(t, action, ct.MISSINGPERMISSION)
	})
}

// Two products share nothing: the same external user ID resolves per product,
// and a product's credential cannot evaluate another product's data.
func TestAccessEvaluationIsScopedToTheProduct(t *testing.T) {
	t.Parallel()
	first := newAccessWorld(t)
	second := newAccessWorld(t)
	sharedExternalID := "user_shared_" + itshared.Faker.UUID().V4()
	firstUser := createDSLProductUser(t, first.product)
	setDSLProductUserExternalID(t, first.product, firstUser.ID, sharedExternalID)
	createDSLMembership(t, first.product, firstUser.ID, first.orgA.Id, first.role.ID)
	secondUser := createDSLProductUser(t, second.product)
	setDSLProductUserExternalID(t, second.product, secondUser.ID, sharedExternalID)
	createDSLMembership(t, second.product, secondUser.ID, second.orgA.Id, second.role.ID)

	t.Run("The same external user ID resolves to each product's own user", func(t *testing.T) {
		response := second.access.User(sharedExternalID).InWorkspace(second.orgA.Id, second.workspaceA).
			Can(second.read).Decide()

		assert.True(t, response.Decision)
		assert.Equal(t, secondUser.ID, response.Context.ProductUser.Id)
	})

	t.Run("A product cannot evaluate another product's organization", func(t *testing.T) {
		response := second.access.User(sharedExternalID).InWorkspace(first.orgA.Id, first.workspaceA).
			Can(first.read).Decide()
		requireDenied(t, response, ct.ORGANIZATIONNOTFOUND)
	})

	t.Run("A product cannot evaluate another product's API key", func(t *testing.T) {
		response := second.access.APIKey(first.apiKey).Can(first.read).Decide()
		requireDenied(t, response, ct.APIKEYINVALID)
	})
}

func TestAccessEvaluationFromThePlatformSession(t *testing.T) {
	t.Parallel()

	t.Run("The product owner's platform session evaluates access, as the admin UI does", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		owner := w.product.AccessClientWith(w.product.OwnerAuthenticatedClient())

		decision := owner.User(w.externalID).InOrganization(w.orgA.Id).Can(w.read).Decide()

		assert.True(t, decision.Decision)
	})

	t.Run("Another tenant's platform session can't evaluate access on the product", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		otherTenant := itdsl.Given(t).Tenant(itdsl.TenantOpts{Alias: "other", Isolated: true}).Build().Tenant("other")
		foreign := w.product.AccessClientWith(otherTenant.OwnerClient)

		resp := foreign.User(w.externalID).InOrganization(w.orgA.Id).Can(w.read).Send()

		assert.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, resp.StatusCode(), string(resp.Body))
	})
}

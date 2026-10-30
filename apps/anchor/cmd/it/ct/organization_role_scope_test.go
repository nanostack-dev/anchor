package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	ct "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// organizationRoleScopeWorld is a product with a project type, a role scoped
// to organization and one scoped to project, an Organization and a product
// user who is not a member yet.
type organizationRoleScopeWorld struct {
	product     *itdsl.ProductContext
	client      *ct.ClientWithResponses
	org         ct.ProductOrganizationResponse
	orgRole     ct.ProductRoleResponse
	projectRole ct.ProductRoleResponse
	user        *itdsl.ProductUser
}

func newOrganizationRoleScopeWorld(t *testing.T) organizationRoleScopeWorld {
	t.Helper()
	product := createTestProductContext(t)
	product.ResourceTypes().CreateUnder("project", "organization")
	return organizationRoleScopeWorld{
		product:     product,
		client:      product.OwnerAuthenticatedClient(),
		org:         product.CreateOrganization(t, "Org-"+uuid.NewString(), nil),
		orgRole:     createScopedRole(t, product, "organization"),
		projectRole: createScopedRole(t, product, "project"),
		user:        createDSLProductUser(t, product),
	}
}

func requireRoleNotOrganizationScoped(t *testing.T, resp statusResponse, body []byte, roleID string) {
	t.Helper()
	metadata := requireAPIError(t, resp, body, http.StatusBadRequest, "ROLE_NOT_ORGANIZATION_SCOPED")
	assert.Equal(t, roleID, metadata["role_id"])
	assert.Equal(t, "project", metadata["resource_type"])
}

func (w organizationRoleScopeWorld) addMember(t *testing.T, roleID string) *ct.AddOrganizationMemberResponse {
	t.Helper()
	resp, err := w.client.AddOrganizationMemberWithResponse(context.Background(), w.product.ProductID, w.org.Id,
		ct.AddOrganizationMemberJSONRequestBody{ProductUserId: w.user.ID, RoleId: new(roleID)})
	require.NoError(t, err)
	return resp
}

func (w organizationRoleScopeWorld) invite(t *testing.T, roleID string) *ct.CreateOrganizationInvitationResponse {
	t.Helper()
	resp, err := w.client.CreateOrganizationInvitationWithResponse(context.Background(), w.product.ProductID,
		w.org.Id, ct.CreateOrganizationInvitationJSONRequestBody{
			Email: openapi_types.Email("invitee-" + uuid.NewString() + "@example.com"), RoleId: new(roleID),
		})
	require.NoError(t, err)
	return resp
}

func TestOrganizationRoleMustBeOrganizationScoped(t *testing.T) {
	t.Parallel()
	t.Run("Adding a member with a role scoped to a project answers 400 and adds no member", func(t *testing.T) {
		t.Parallel()
		w := newOrganizationRoleScopeWorld(t)

		resp := w.addMember(t, w.projectRole.Id)

		requireRoleNotOrganizationScoped(t, resp, resp.Body, w.projectRole.Id)
		got, err := w.client.GetOrganizationMemberWithResponse(context.Background(), w.product.ProductID, w.org.Id,
			w.user.ID, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, got.StatusCode(), string(got.Body))
	})

	t.Run(
		"Changing a member's role to one scoped to a project answers 400 and keeps the current role",
		func(t *testing.T) {
			t.Parallel()
			w := newOrganizationRoleScopeWorld(t)
			added := w.addMember(t, w.orgRole.Id)
			require.Equal(t, http.StatusCreated, added.StatusCode(), string(added.Body))

			resp, err := w.client.UpdateOrganizationMemberRoleWithResponse(context.Background(), w.product.ProductID,
				w.org.Id, w.user.ID, ct.UpdateOrganizationMemberRoleJSONRequestBody{RoleId: new(w.projectRole.Id)})
			require.NoError(t, err)

			requireRoleNotOrganizationScoped(t, resp, resp.Body, w.projectRole.Id)
			got, err := w.client.GetOrganizationMemberWithResponse(context.Background(), w.product.ProductID, w.org.Id,
				w.user.ID, nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, got.StatusCode(), string(got.Body))
			require.NotNil(t, got.JSON200.Role)
			assert.Equal(t, w.orgRole.Id, got.JSON200.Role.Id)
		},
	)

	t.Run("Inviting with a role scoped to a project answers 400", func(t *testing.T) {
		t.Parallel()
		w := newOrganizationRoleScopeWorld(t)

		resp := w.invite(t, w.projectRole.Id)

		requireRoleNotOrganizationScoped(t, resp, resp.Body, w.projectRole.Id)
	})

	t.Run(
		"Changing an invitation's role to one scoped to a project answers 400 and keeps the current role",
		func(t *testing.T) {
			t.Parallel()
			w := newOrganizationRoleScopeWorld(t)
			invited := w.invite(t, w.orgRole.Id)
			require.Equal(t, http.StatusCreated, invited.StatusCode(), string(invited.Body))
			invitation := *invited.JSON201

			resp, err := w.client.UpdateOrganizationInvitationWithResponse(context.Background(), w.product.ProductID,
				w.org.Id, invitation.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
					RoleId: new(w.projectRole.Id), ExpiresAt: time.Now().Add(48 * time.Hour),
				})
			require.NoError(t, err)

			requireRoleNotOrganizationScoped(t, resp, resp.Body, w.projectRole.Id)
			got, err := w.client.GetOrganizationInvitationWithResponse(context.Background(), w.product.ProductID,
				w.org.Id, invitation.Id)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, got.StatusCode(), string(got.Body))
			assert.Equal(t, new(w.orgRole.Id), got.JSON200.RoleId)
		},
	)

	t.Run(
		"Creating an organization whose founding member holds a role scoped to a project answers 400",
		func(t *testing.T) {
			t.Parallel()
			w := newOrganizationRoleScopeWorld(t)

			resp := w.product.Organizations().CreateRaw(ct.CreateProductOrganizationJSONRequestBody{
				Name:           "Org-" + uuid.NewString(),
				FoundingMember: &ct.FoundingMemberRequest{ProductUserId: w.user.ID, RoleId: new(w.projectRole.Id)},
			})

			requireRoleNotOrganizationScoped(t, resp, resp.Body, w.projectRole.Id)
		},
	)

	t.Run("A custom role scoped to a project can't be an Organization role", func(t *testing.T) {
		t.Parallel()
		w := newOrganizationRoleScopeWorld(t)
		project := w.product.ResourceTypes().BySlug("project")
		w.product.ResourceTypes().Update(project.Id, ct.UpdateProductResourceTypeJSONRequestBody{
			AllowsCustomRoles: new(true),
		})
		created, err := w.client.CreateOrganizationRoleWithResponse(context.Background(), w.product.ProductID,
			w.org.Id, ct.CreateOrganizationRoleJSONRequestBody{Name: "Release manager", ResourceType: "project"})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))

		resp := w.addMember(t, created.JSON201.Id)

		requireRoleNotOrganizationScoped(t, resp, resp.Body, created.JSON201.Id)
	})
}

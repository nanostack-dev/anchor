package ct_test

import (
	"net/http"
	"testing"

	itshared "anchor/cmd/it/shared"

	ct "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A member may hold no Organization role (ADR-0022 D5): they belong to the
// Organization and hold no permission there.
func TestOrganizationMemberWithoutARole(t *testing.T) {
	t.Parallel()

	t.Run("A member added without a role lists with a null role", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		user := createDSLProductUser(t, w.product)
		members := w.product.Members(w.orgA.Id)

		added := members.Add(user.ID, nil)

		assert.Nil(t, added.Role)
		assert.Nil(t, members.Get(user.ID).Role)
		assert.Nil(t, members.Organization(user.ID).Role)
	})

	t.Run("A member without a role reaches the organization and no action", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		user := createDSLProductUser(t, w.product)
		externalID := "user_" + itshared.Faker.UUID().V4()
		setDSLProductUserExternalID(t, w.product, user.ID, externalID)
		w.product.Members(w.orgA.Id).Add(user.ID, nil)
		inOrganization := w.access.User(externalID).InOrganization(w.orgA.Id)

		reach := inOrganization.Decide()
		action := inOrganization.Can(w.read).Decide()

		assert.True(t, reach.Decision)
		assert.Nil(t, reach.Context.Role)
		assert.Empty(t, *reach.Context.Permissions)
		requireDenied(t, action, ct.MISSINGPERMISSION)
	})

	t.Run("A role update with a null role removes the role and its permissions", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		members := w.product.Members(w.orgA.Id)

		updated := members.SetRole(w.userID, nil)
		action := w.access.User(w.externalID).InOrganization(w.orgA.Id).Can(w.read).Decide()

		assert.Nil(t, updated.Role)
		assert.Nil(t, members.Get(w.userID).Role)
		requireDenied(t, action, ct.MISSINGPERMISSION)
	})

	t.Run("A role update gives a member without a role its role back", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		user := createDSLProductUser(t, w.product)
		members := w.product.Members(w.orgA.Id)
		members.Add(user.ID, nil)

		updated := members.SetRole(user.ID, new(w.role.ID))

		require.NotNil(t, updated.Role)
		assert.Equal(t, w.role.ID, updated.Role.Id)
	})

	t.Run("A role update that leaves role_id out answers bad request", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)

		resp := w.product.Members(w.orgA.Id).SetRoleRawBody(w.userID, `{}`)

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		require.NotEmpty(t, resp.JSON400.Errors)
		assert.Equal(t, "VALIDATION_ERROR", resp.JSON400.Errors[0].Code)
		assert.Equal(t, new("role_id"), resp.JSON400.Errors[0].Field)
		require.NotNil(t, w.product.Members(w.orgA.Id).Get(w.userID).Role)
	})

	t.Run("A founding member added without a role holds none", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		user := createDSLProductUser(t, w.product)

		org := w.product.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{
			Name:           "Org-" + uuid.NewString(),
			FoundingMember: &ct.FoundingMemberRequest{ProductUserId: user.ID},
		})

		assert.Nil(t, w.product.Members(org.Id).Get(user.ID).Role)
	})
}

func TestOrganizationInvitationWithoutARole(t *testing.T) {
	t.Parallel()

	t.Run("An invitation without a role accepts as a member without one", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		email := uniqueEmail()
		productUserID := w.newProductUser(email)

		invitation := w.invitations.Create(w.organizationID, ct.CreateOrganizationInvitationJSONRequestBody{
			Email: openapi_types.Email(email),
		})
		w.accept(invitation.Id, productUserID)

		assert.Nil(t, invitation.RoleId)
		assert.Nil(t, w.invitations.Get(w.organizationID, invitation.Id).RoleId)
		assert.Nil(t, w.product.Members(w.organizationID).Get(productUserID).Role)
	})

	t.Run("An invitation update with a null role invites with no role", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		invitation := w.invite(uniqueEmail())

		updated := w.invitations.Update(w.organizationID, invitation.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
			ExpiresAt: invitation.ExpiresAt,
		})

		assert.Nil(t, updated.RoleId)
	})

	t.Run("An invitation update that leaves role_id out answers bad request", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		invitation := w.invite(uniqueEmail())

		resp := w.invitations.UpdateRawBody(w.organizationID, invitation.Id,
			`{"expires_at":"`+invitation.ExpiresAt.Format("2006-01-02T15:04:05.999999999Z07:00")+`"}`)

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		require.NotEmpty(t, resp.JSON400.Errors)
		assert.Equal(t, new("role_id"), resp.JSON400.Errors[0].Field)
		assert.Equal(t, w.roleID, *w.invitations.Get(w.organizationID, invitation.Id).RoleId)
	})
}

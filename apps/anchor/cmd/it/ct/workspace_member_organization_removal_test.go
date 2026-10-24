package ct_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveOrganizationMember_RemovesTheirWorkspaceRoles(t *testing.T) {
	t.Parallel()

	t.Run("Removing an organization member removes their role in all its workspaces", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		otherWorkspaceID := w.newWorkspace(w.organizationID)
		productUserID := w.newOrganizationMember()
		w.members.Assign(w.workspaceID, productUserID, w.roleID)
		w.members.Assign(otherWorkspaceID, productUserID, w.roleID)

		w.removeOrganizationMember(productUserID)

		for _, workspaceID := range []string{w.workspaceID, otherWorkspaceID} {
			resp := w.members.GetRaw(workspaceID, productUserID)
			require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
			assert.Equal(t, "WORKSPACE_MEMBERSHIP_NOT_FOUND", errorCode(t, resp.JSON404.Errors))
		}
	})

	t.Run("Removing an organization member keeps their workspace roles in other organizations", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()
		w.members.Assign(w.workspaceID, productUserID, w.roleID)
		otherOrganizationID := w.newOrganization()
		otherWorkspaceID := w.newWorkspace(otherOrganizationID)
		createDSLMembership(t, w.product, productUserID, otherOrganizationID, w.roleID)
		otherMembers := w.product.WorkspaceMembers(otherOrganizationID)
		otherMembers.Assign(otherWorkspaceID, productUserID, w.roleID)

		w.removeOrganizationMember(productUserID)

		assert.Equal(t, w.roleID, otherMembers.Get(otherWorkspaceID, productUserID).Role.Id)
	})

	t.Run("Removing an organization member emits a deleted event for each workspace role", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		sink := w.product.CaptureEvents()
		otherWorkspaceID := w.newWorkspace(w.organizationID)
		otherRoleID := w.newRole()
		productUserID := w.newOrganizationMember()
		w.members.Assign(w.workspaceID, productUserID, w.roleID)
		w.members.Assign(otherWorkspaceID, productUserID, otherRoleID)

		w.removeOrganizationMember(productUserID)

		sink.WaitFor(eventWorkspaceMembershipDeleted, w.membershipFields(productUserID, w.roleID))
		sink.WaitFor(eventWorkspaceMembershipDeleted, map[string]string{
			"organization_id": w.organizationID,
			"workspace_id":    otherWorkspaceID,
			"product_user_id": productUserID,
			"role_id":         otherRoleID,
		})
	})

	t.Run("A member who rejoins the organization holds no workspace role", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()
		w.members.Assign(w.workspaceID, productUserID, w.roleID)

		w.removeOrganizationMember(productUserID)
		w.addMember(productUserID)

		assert.Equal(t, http.StatusNotFound, w.members.GetRaw(w.workspaceID, productUserID).StatusCode())
	})
}

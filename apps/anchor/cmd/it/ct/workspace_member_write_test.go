package ct_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignWorkspaceMemberRole(t *testing.T) {
	t.Parallel()

	t.Run("Assigning a role gives an organization member that role in the workspace", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()

		assigned := w.members.Assign(w.workspaceID, productUserID, w.roleID)

		assert.Equal(t, w.workspaceID, assigned.WorkspaceId)
		assert.Equal(t, w.organizationID, assigned.OrganizationId)
		assert.Equal(t, productUserID, assigned.ProductUserId)
		assert.Equal(t, w.roleID, assigned.Role.Id)
		assert.NotEmpty(t, assigned.Role.Name)
		assert.NotEmpty(t, assigned.Email)
		assert.False(t, assigned.AssignedAt.IsZero())
		assert.Equal(t, assigned, w.members.Get(w.workspaceID, productUserID))
	})

	t.Run("Assigning another role replaces the role and keeps the assigned time", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()
		first := w.members.Assign(w.workspaceID, productUserID, w.roleID)
		otherRoleID := w.newRole()

		replaced := w.members.Assign(w.workspaceID, productUserID, otherRoleID)

		assert.Equal(t, otherRoleID, replaced.Role.Id)
		assert.True(t, first.AssignedAt.Equal(replaced.AssignedAt))
		assert.Equal(t, otherRoleID, w.members.Get(w.workspaceID, productUserID).Role.Id)
	})

	t.Run("Assigning the role the member holds already writes nothing", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()
		first := w.members.Assign(w.workspaceID, productUserID, w.roleID)
		updatedAt := w.workspaceRoleUpdatedAt(productUserID)

		second := w.members.Assign(w.workspaceID, productUserID, w.roleID)

		assert.Equal(t, first, second)
		assert.Equal(t, updatedAt, w.workspaceRoleUpdatedAt(productUserID))
	})

	t.Run("A member holds a separate role in each workspace", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()
		otherWorkspaceID := w.newWorkspace(w.organizationID)
		otherRoleID := w.newRole()

		w.members.Assign(w.workspaceID, productUserID, w.roleID)
		w.members.Assign(otherWorkspaceID, productUserID, otherRoleID)

		assert.Equal(t, w.roleID, w.members.Get(w.workspaceID, productUserID).Role.Id)
		assert.Equal(t, otherRoleID, w.members.Get(otherWorkspaceID, productUserID).Role.Id)
	})
}

func TestGetWorkspaceMember(t *testing.T) {
	t.Parallel()

	t.Run("Getting a member who holds no role in the workspace answers not found", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()

		resp := w.members.GetRaw(w.workspaceID, productUserID)

		require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
		assert.Equal(t, "WORKSPACE_MEMBERSHIP_NOT_FOUND", errorCode(t, resp.JSON404.Errors))
	})
}

func TestRemoveWorkspaceMember(t *testing.T) {
	t.Parallel()

	t.Run("Removing a workspace role keeps the organization membership", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()
		w.members.Assign(w.workspaceID, productUserID, w.roleID)

		w.members.Remove(w.workspaceID, productUserID)

		assert.Equal(t, http.StatusNotFound, w.members.GetRaw(w.workspaceID, productUserID).StatusCode())
		assert.Equal(t, http.StatusOK, w.memberRaw(w.organizationID, productUserID).StatusCode())
	})

	t.Run("Removing a role the member does not hold answers not found", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()

		resp := w.members.RemoveRaw(w.workspaceID, productUserID)

		require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
		assert.Equal(t, "WORKSPACE_MEMBERSHIP_NOT_FOUND", errorCode(t, resp.JSON404.Errors))
	})
}

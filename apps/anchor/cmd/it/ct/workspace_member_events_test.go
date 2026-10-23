package ct_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	eventWorkspaceMembershipCreated = "workspace.membership.created"
	eventWorkspaceMembershipUpdated = "workspace.membership.updated"
	eventWorkspaceMembershipDeleted = "workspace.membership.deleted"
)

func (w workspaceMemberWorld) membershipFields(productUserID, roleID string) map[string]string {
	return map[string]string{
		"organization_id": w.organizationID,
		"workspace_id":    w.workspaceID,
		"product_user_id": productUserID,
		"role_id":         roleID,
	}
}

func TestWorkspaceMemberEvents(t *testing.T) {
	t.Parallel()

	t.Run("Assigning a first role emits the created event", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		sink := w.product.CaptureEvents()
		productUserID := w.newOrganizationMember()

		w.members.Assign(w.workspaceID, productUserID, w.roleID)

		sink.WaitFor(eventWorkspaceMembershipCreated, w.membershipFields(productUserID, w.roleID))
	})

	t.Run("Replacing the role emits the updated event with the new role", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		sink := w.product.CaptureEvents()
		productUserID := w.newOrganizationMember()
		otherRoleID := w.newRole()
		w.members.Assign(w.workspaceID, productUserID, w.roleID)

		w.members.Assign(w.workspaceID, productUserID, otherRoleID)

		sink.WaitFor(eventWorkspaceMembershipUpdated, w.membershipFields(productUserID, otherRoleID))
	})

	t.Run("Assigning the role already held emits no event", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		sink := w.product.CaptureEvents()
		productUserID := w.newOrganizationMember()
		marker := w.newOrganizationMember()
		w.members.Assign(w.workspaceID, productUserID, w.roleID)

		w.members.Assign(w.workspaceID, productUserID, w.roleID)
		w.members.Assign(w.workspaceID, marker, w.roleID)

		sink.WaitFor(eventWorkspaceMembershipCreated, w.membershipFields(marker, w.roleID))
		assert.Equal(t, 2, sink.Count(eventWorkspaceMembershipCreated))
		assert.Zero(t, sink.Count(eventWorkspaceMembershipUpdated))
	})

	t.Run("Removing the role emits the deleted event with the role that was held", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		sink := w.product.CaptureEvents()
		productUserID := w.newOrganizationMember()
		w.members.Assign(w.workspaceID, productUserID, w.roleID)

		w.members.Remove(w.workspaceID, productUserID)

		sink.WaitFor(eventWorkspaceMembershipDeleted, w.membershipFields(productUserID, w.roleID))
	})

	t.Run("A refused assign emits no event", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		sink := w.product.CaptureEvents()
		outsiderID := w.newProductUser(uniqueEmail())
		marker := w.newOrganizationMember()

		w.members.AssignRaw(w.workspaceID, outsiderID, w.roleID)
		w.members.Assign(w.workspaceID, marker, w.roleID)

		sink.WaitFor(eventWorkspaceMembershipCreated, w.membershipFields(marker, w.roleID))
		assert.Equal(t, 1, sink.Count(eventWorkspaceMembershipCreated))
	})
}

package ct_test

import (
	"net/http"
	"slices"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const concurrentWorkspaceAssigns = 8

// assignStatus assigns through the generated client, which is safe to call from
// a goroutine: the DSL acts fail the test with require.
func (w workspaceMemberWorld) assignStatus(productUserID string) (int, error) {
	resp, err := w.product.AllScopeAPIKeyClient().AssignWorkspaceMemberRoleWithResponse(
		w.t.Context(), w.product.ProductID, w.organizationID, w.workspaceID, productUserID,
		ct.WorkspaceMemberRoleRequest{RoleId: w.roleID},
	)
	if err != nil {
		return 0, err
	}
	return resp.StatusCode(), nil
}

func TestWorkspaceMemberWrites_Concurrent(t *testing.T) {
	t.Parallel()

	t.Run("Concurrent assigns of the same role create one membership and one created event", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		sink := w.product.CaptureEvents()
		productUserID := w.newOrganizationMember()

		statuses, err := runConcurrently(
			slices.Repeat([]string{productUserID}, concurrentWorkspaceAssigns), w.assignStatus,
		)

		require.NoError(t, err)
		assert.Equal(t, slices.Repeat([]int{http.StatusOK}, concurrentWorkspaceAssigns), statuses)
		sink.WaitFor(eventWorkspaceMembershipCreated, w.membershipFields(productUserID, w.roleID))
		marker := w.newOrganizationMember()
		w.members.Assign(w.workspaceID, marker, w.roleID)
		sink.WaitFor(eventWorkspaceMembershipCreated, w.membershipFields(marker, w.roleID))
		assert.Equal(t, 2, sink.Count(eventWorkspaceMembershipCreated))
		assert.Zero(t, sink.Count(eventWorkspaceMembershipUpdated))
	})

	t.Run("An assign racing the removal from the organization never leaves a role behind", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()
		removeStatus := func(string) (int, error) {
			resp, err := w.product.AllScopeAPIKeyClient().RemoveOrganizationMemberWithResponse(
				t.Context(), w.product.ProductID, w.organizationID, productUserID,
			)
			if err != nil {
				return 0, err
			}
			return resp.StatusCode(), nil
		}

		statuses, err := runConcurrently([]string{"assign", "remove"}, func(action string) (int, error) {
			if action == "assign" {
				return w.assignStatus(productUserID)
			}
			return removeStatus(action)
		})

		require.NoError(t, err)
		assert.Contains(t, []int{http.StatusOK, http.StatusConflict}, statuses[0])
		assert.Equal(t, http.StatusNoContent, statuses[1])
		assert.Equal(t, http.StatusNotFound, w.members.GetRaw(w.workspaceID, productUserID).StatusCode())
	})
}

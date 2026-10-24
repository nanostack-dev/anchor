package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// workspaceRoleWorld adds to accessWorld an editor role that grants update, and
// a second workspace in organization A where the member holds no workspace role.
type workspaceRoleWorld struct {
	accessWorld
	editor     string
	workspace2 string
}

func newWorkspaceRoleWorld(t *testing.T) workspaceRoleWorld {
	t.Helper()
	w := newAccessWorld(t)
	editor := createDSLProductRoleWithPermissions(
		t,
		w.product,
		"editor-"+uuid.NewString(),
		nil,
		[]string{w.read, w.update},
	)
	return workspaceRoleWorld{accessWorld: w, editor: editor.ID, workspace2: w.workspace(w.orgA.Id)}
}

func (w workspaceRoleWorld) assign(t *testing.T, workspaceID, roleID string) {
	t.Helper()
	resp, err := w.product.AllScopeAPIKeyClient().AssignWorkspaceMemberRoleWithResponse(
		t.Context(), w.product.ProductID, w.orgA.Id, workspaceID, w.userID,
		ct.AssignWorkspaceMemberRoleJSONRequestBody{RoleId: roleID},
	)
	require.NoError(t, err)
	requireStatus(t, http.StatusOK, resp.StatusCode(), resp.Body)
}

func (w workspaceRoleWorld) unassign(t *testing.T, workspaceID string) {
	t.Helper()
	resp, err := w.product.AllScopeAPIKeyClient().RemoveWorkspaceMemberWithResponse(
		t.Context(), w.product.ProductID, w.orgA.Id, workspaceID, w.userID,
	)
	require.NoError(t, err)
	requireStatus(t, http.StatusNoContent, resp.StatusCode(), resp.Body)
}

func TestAccessEvaluationWithWorkspaceRoles(t *testing.T) {
	t.Parallel()

	t.Run("A workspace role adds its permissions in that workspace only", func(t *testing.T) {
		t.Parallel()
		w := newWorkspaceRoleWorld(t)
		w.assign(t, w.workspaceA, w.editor)

		decision := w.memberInWorkspaceA().Can(w.read, w.update).Decide()
		require.True(t, decision.Decision)
		require.NotNil(t, decision.Context.WorkspaceRole)
		assert.Equal(t, w.editor, decision.Context.WorkspaceRole.Id)
		assert.Equal(t, w.role.ID, decision.Context.Role.Id, "the organization role is still reported")
		assert.ElementsMatch(t, []string{w.read, w.update}, *decision.Context.Permissions)

		otherWorkspace := w.access.User(w.externalID).InWorkspace(w.orgA.Id, w.workspace2).Can(w.update).Decide()
		requireDenied(t, otherWorkspace, ct.MISSINGPERMISSION)
		assert.Nil(t, otherWorkspace.Context.WorkspaceRole)

		requireDenied(t, w.access.User(w.externalID).InOrganization(w.orgA.Id).Can(w.update).Decide(),
			ct.MISSINGPERMISSION)
	})

	t.Run("A removed workspace role takes its permissions away on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newWorkspaceRoleWorld(t)
		w.assign(t, w.workspaceA, w.editor)
		require.True(t, w.memberInWorkspaceA().Can(w.update).Decide().Decision)

		w.unassign(t, w.workspaceA)

		requireDenied(t, w.memberInWorkspaceA().Can(w.update).Decide(), ct.MISSINGPERMISSION)
		assert.True(t, w.memberInWorkspaceA().Can(w.read).Decide().Decision, "the organization role still grants read")
	})

	t.Run("A replaced workspace role applies on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newWorkspaceRoleWorld(t)
		w.assign(t, w.workspaceA, w.editor)
		require.True(t, w.memberInWorkspaceA().Can(w.update).Decide().Decision)

		w.assign(t, w.workspaceA, w.role.ID)

		decision := w.memberInWorkspaceA().Can(w.update).Decide()
		requireDenied(t, decision, ct.MISSINGPERMISSION)
		assert.Equal(t, w.role.ID, decision.Context.WorkspaceRole.Id)
	})

	t.Run("A member removed from the organization loses their workspace roles", func(t *testing.T) {
		t.Parallel()
		w := newWorkspaceRoleWorld(t)
		w.assign(t, w.workspaceA, w.editor)

		resp, err := w.product.AllScopeAPIKeyClient().RemoveOrganizationMemberWithResponse(
			t.Context(), w.product.ProductID, w.orgA.Id, w.userID)
		require.NoError(t, err)
		requireStatus(t, http.StatusNoContent, resp.StatusCode(), resp.Body)
		requireDenied(t, w.memberInWorkspaceA().Can(w.read).Decide(), ct.NOTAMEMBER)

		// Through the API, not the DSL: the DSL writes the row without the event a
		// service emits, and the event is what tells the relationship store.
		rejoin, err := w.product.AllScopeAPIKeyClient().AddOrganizationMemberWithResponse(
			t.Context(), w.product.ProductID, w.orgA.Id,
			ct.AddOrganizationMemberJSONRequestBody{ProductUserId: w.userID, RoleId: w.role.ID})
		require.NoError(t, err)
		requireStatus(t, http.StatusCreated, rejoin.StatusCode(), rejoin.Body)

		decision := w.memberInWorkspaceA().Can(w.update).Decide()
		requireDenied(t, decision, ct.MISSINGPERMISSION)
		assert.Nil(t, decision.Context.WorkspaceRole, "rejoining does not bring the old workspace role back")
	})

	t.Run("A permission added to the workspace role applies on the next evaluation", func(t *testing.T) {
		t.Parallel()
		w := newWorkspaceRoleWorld(t)
		w.assign(t, w.workspaceA, w.editor)
		requireDenied(t, w.memberInWorkspaceA().Can(w.remove).Decide(), ct.MISSINGPERMISSION)

		resp, err := w.product.AllScopeAPIKeyClient().AssignPermissionToProductRoleWithResponse(
			t.Context(), w.product.ProductID, w.editor,
			ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: w.remove})
		require.NoError(t, err)
		require.Less(t, resp.StatusCode(), 300, string(resp.Body))

		assert.True(t, w.memberInWorkspaceA().Can(w.remove).Decide().Decision)
		requireDenied(t, w.access.User(w.externalID).InOrganization(w.orgA.Id).Can(w.remove).Decide(),
			ct.MISSINGPERMISSION)
	})

	t.Run("The decision is read from the product's OpenFGA store", func(t *testing.T) {
		t.Parallel()
		w := newWorkspaceRoleWorld(t)
		w.assign(t, w.workspaceA, w.editor)
		require.True(t, w.memberInWorkspaceA().Can(w.update).Decide().Decision)

		var tuples int
		require.NoError(t, testDB.QueryRowContext(t.Context(), `
			SELECT count(*)
			FROM openfga.tuple t
			JOIN authz_stores s ON s.store_id = t.store
			WHERE s.product_id = $1 AND t.object_type = 'workspace' AND t.object_id = $2
			  AND t._user = 'user:' || $3`,
			w.product.ProductID, w.workspaceA, w.userID,
		).Scan(&tuples))
		assert.Equal(t, 1, tuples, "the member's workspace role is a tuple in the product's store")
	})
}

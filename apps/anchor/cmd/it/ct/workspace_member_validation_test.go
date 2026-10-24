package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceMemberRoutes_RefuseAWorkspaceTheOrganizationDoesNotHave(t *testing.T) {
	t.Parallel()

	foreignWorkspaces := map[string]func(t *testing.T, w workspaceMemberWorld) string{
		"a workspace that does not exist": func(*testing.T, workspaceMemberWorld) string {
			return ids.MustNew("ws")
		},
		"a workspace of another organization": func(_ *testing.T, w workspaceMemberWorld) string {
			return w.newWorkspace(w.newOrganization())
		},
		"a workspace of another product": func(t *testing.T, _ workspaceMemberWorld) string {
			return newWorkspaceMemberWorld(t).workspaceID
		},
	}

	for name, foreignWorkspace := range foreignWorkspaces {
		t.Run("Every workspace member route answers not found for "+name, func(t *testing.T) {
			t.Parallel()

			w := newWorkspaceMemberWorld(t)
			productUserID := w.newOrganizationMember()
			workspaceID := foreignWorkspace(t, w)

			statuses := map[string]int{
				"assign": w.members.AssignRaw(workspaceID, productUserID, w.roleID).StatusCode(),
				"get":    w.members.GetRaw(workspaceID, productUserID).StatusCode(),
				"remove": w.members.RemoveRaw(workspaceID, productUserID).StatusCode(),
				"search": w.members.SearchRaw(workspaceID, ct.SearchWorkspaceMembersJSONRequestBody{}).StatusCode(),
			}

			for route, status := range statuses {
				assert.Equal(t, http.StatusNotFound, status, route)
			}
			assert.Equal(t, "WORKSPACE_NOT_FOUND",
				errorCode(t, w.members.AssignRaw(workspaceID, productUserID, w.roleID).JSON404.Errors))
		})
	}
}

func TestAssignWorkspaceMemberRole_RefusesARoleOfAnotherProduct(t *testing.T) {
	t.Parallel()

	t.Run("Assigning a role of another product is refused and writes nothing", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()
		foreignRoleID := newWorld(t).roleID

		resp := w.members.AssignRaw(w.workspaceID, productUserID, foreignRoleID)

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assert.Equal(t, "ROLE_NOT_FOUND_IN_REQUEST", errorCode(t, resp.JSON400.Errors))
		assert.Equal(t, http.StatusNotFound, w.members.GetRaw(w.workspaceID, productUserID).StatusCode())
	})

	t.Run("Assigning a blank role is refused", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		productUserID := w.newOrganizationMember()

		resp := w.members.AssignRaw(w.workspaceID, productUserID, "")

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	})
}

func TestAssignWorkspaceMemberRole_RefusesAUserWhoIsNotAnOrganizationMember(t *testing.T) {
	t.Parallel()

	t.Run("Assigning a role to a product user outside the organization is refused", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		outsiderID := w.newProductUser(uniqueEmail())

		resp := w.members.AssignRaw(w.workspaceID, outsiderID, w.roleID)

		require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
		assert.Equal(t, "WORKSPACE_MEMBER_NOT_IN_ORGANIZATION", errorCode(t, resp.JSON409.Errors))
		assert.Equal(t, http.StatusNotFound, w.members.GetRaw(w.workspaceID, outsiderID).StatusCode())
	})

	t.Run("Assigning a role to a member of another organization is refused", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		otherOrganizationID := w.newOrganization()
		productUserID := w.newProductUser(uniqueEmail())
		createDSLMembership(t, w.product, productUserID, otherOrganizationID, w.roleID)

		resp := w.members.AssignRaw(w.workspaceID, productUserID, w.roleID)

		require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
		assert.Equal(t, "WORKSPACE_MEMBER_NOT_IN_ORGANIZATION", errorCode(t, resp.JSON409.Errors))
	})
}

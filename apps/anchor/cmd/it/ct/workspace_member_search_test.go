package ct_test

import (
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assignNewMembers gives count new organization members the world's role in the
// workspace, and returns their product user ids.
func (w workspaceMemberWorld) assignNewMembers(count int) []string {
	w.t.Helper()
	productUserIDs := make([]string, count)
	for i := range productUserIDs {
		productUserIDs[i] = w.newOrganizationMember()
		w.members.Assign(w.workspaceID, productUserIDs[i], w.roleID)
	}
	return productUserIDs
}

func TestSearchWorkspaceMembers(t *testing.T) {
	t.Parallel()

	t.Run("Searching lists only the members of the workspace", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		inWorkspace := w.assignNewMembers(2)
		otherWorkspaceID := w.newWorkspace(w.organizationID)
		w.members.Assign(otherWorkspaceID, w.newOrganizationMember(), w.roleID)

		page := w.members.Search(w.workspaceID, ct.SearchWorkspaceMembersJSONRequestBody{})

		assert.ElementsMatch(t, inWorkspace, workspaceMemberUserIDs(page.Items))
		assert.EqualValues(t, 2, page.Total)
	})

	t.Run("Searching pages through every member once with the cursor", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		assigned := w.assignNewMembers(5)

		var seen []string
		request := ct.SearchWorkspaceMembersJSONRequestBody{Pagination: &ct.PaginationRequest{Limit: new(int32(2))}}
		for pages := 0; ; pages++ {
			require.Less(t, pages, len(assigned), "the cursor never reached the last page")
			page := w.members.Search(w.workspaceID, request)
			seen = append(seen, workspaceMemberUserIDs(page.Items)...)
			if page.NextCursor == nil {
				break
			}
			request.Pagination.Cursor = page.NextCursor
		}

		assert.ElementsMatch(t, assigned, seen)
	})

	t.Run("Searching filters by product user and by role", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceMemberWorld(t)
		assigned := w.assignNewMembers(2)
		otherRoleID := w.newRole()
		withOtherRole := w.newOrganizationMember()
		w.members.Assign(w.workspaceID, withOtherRole, otherRoleID)

		byUser := w.members.Search(w.workspaceID, ct.SearchWorkspaceMembersJSONRequestBody{
			Filter: &ct.WorkspaceMemberFilter{ProductUserIds: &assigned},
		})
		byRole := w.members.Search(w.workspaceID, ct.SearchWorkspaceMembersJSONRequestBody{
			Filter: &ct.WorkspaceMemberFilter{RoleIds: &[]string{otherRoleID}},
		})

		assert.ElementsMatch(t, assigned, workspaceMemberUserIDs(byUser.Items))
		assert.Equal(t, []string{withOtherRole}, workspaceMemberUserIDs(byRole.Items))
	})
}

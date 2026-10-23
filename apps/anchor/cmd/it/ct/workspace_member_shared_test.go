package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// workspaceMemberWorld is a product with one organization, one workspace in it
// and a role to hold there.
type workspaceMemberWorld struct {
	world
	members     itdsl.WorkspaceMemberClient
	workspaceID string
}

func newWorkspaceMemberWorld(t *testing.T) workspaceMemberWorld {
	t.Helper()
	w := newWorld(t)
	return workspaceMemberWorld{
		world:       w,
		members:     w.product.WorkspaceMembers(w.organizationID),
		workspaceID: w.newWorkspace(w.organizationID),
	}
}

func (w world) newWorkspace(organizationID string) string {
	w.t.Helper()
	return w.product.Workspaces(organizationID).
		Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "ws-" + ids.MustNew("ct")}).Id
}

// newOrganizationMember adds a new product user to the world's organization.
func (w world) newOrganizationMember() string {
	w.t.Helper()
	productUserID := w.newProductUser(uniqueEmail())
	w.addMember(productUserID)
	return productUserID
}

func (w world) removeOrganizationMemberRaw(productUserID string) *ct.RemoveOrganizationMemberResponse {
	w.t.Helper()
	resp, err := w.product.AllScopeAPIKeyClient().RemoveOrganizationMemberWithResponse(
		w.t.Context(), w.product.ProductID, w.organizationID, productUserID,
	)
	require.NoError(w.t, err)
	return resp
}

func (w world) removeOrganizationMember(productUserID string) {
	w.t.Helper()
	resp := w.removeOrganizationMemberRaw(productUserID)
	require.Equal(w.t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
}

func (w workspaceMemberWorld) workspaceRoleUpdatedAt(productUserID string) string {
	w.t.Helper()
	var updatedAt string
	require.NoError(w.t, testDB.QueryRow(
		"SELECT updated_at::text FROM workspace_memberships WHERE workspace_id = $1 AND product_user_id = $2",
		w.workspaceID, productUserID,
	).Scan(&updatedAt))
	return updatedAt
}

func workspaceMemberUserIDs(items []ct.WorkspaceMemberResponse) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ProductUserId
	}
	return ids
}

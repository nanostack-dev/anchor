package ct_test

import (
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// PR 287 marks the resource that mirrors a built-in workspace, and treats a
// marked resource whose workspace row is gone as absent, but only when the
// decision names "type": "workspace" (reachWorkspace). A workspace the
// previous release deletes during the rolling deploy leaves its marked
// resource, its registered descendants and the role assignments on it until
// the next start's backfill. Meanwhile a decision on a resource below it
// (registered, or unregistered and named under it) still walks the stale
// workspace, so a member whose role was held on the deleted workspace keeps
// access to everything under it.
func TestSecurityDeletedWorkspaceGrantsNothingBelowIt(t *testing.T) {
	t.Parallel()

	t.Run("A role on a workspace the previous release deleted gives nothing on the resources under it",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			product.ResourceTypes().CreateUnder(workspaceType, "organization")
			product.ResourceTypes().CreateUnder("document", workspaceType)
			documentView := "document:view"
			product.CreateScopedPermissions(t, "document", documentView)
			orgMember := createRoleTemplate(t, product, ct.CreateProductRoleJSONRequestBody{
				Name: "Member " + ids.MustNew("ct"),
			})
			editor := createRoleTemplate(t, product, rolePermitting(workspaceType, "Workspace editor", documentView))
			org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
			user := createDSLProductUser(t, product)
			externalID := "user_" + ids.MustNew("ext")
			setDSLProductUserExternalID(t, product, user.ID, externalID)
			addMember(t, product, org.Id, user.ID, orgMember.Id)
			workspace := product.Workspaces(org.Id).Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Payments"})
			document := resourceID()
			product.Resources(org.Id).Put("document", document, "Payroll", itdsl.Ref(workspaceType, workspace.Id))
			product.ResourceMembers(org.Id, workspaceType, workspace.Id).Put(user.ID, editor.Id)
			as := product.Access().User(externalID)
			requireAllowed(t, as.OnResource(org.Id, "document", document).Can(documentView).Decide())

			// The previous release deletes the workspace row only.
			_, err := testDB.ExecContext(t.Context(), `DELETE FROM workspaces WHERE id = $1`, workspace.Id)
			require.NoError(t, err)

			requireDenied(t, as.InWorkspace(org.Id, workspace.Id).Can(documentView).Decide(),
				ct.WORKSPACENOTINORGANIZATION)
			registered := as.OnResource(org.Id, "document", document).Can(documentView).Decide()
			assert.False(t, registered.Decision, "registered document under the deleted workspace: %+v",
				registered.Context)
			unregistered := as.OnResource(org.Id, "document", resourceID()).Under(workspaceType, workspace.Id).
				Can(documentView).Decide()
			assert.False(t, unregistered.Decision, "unregistered document named under the deleted workspace: %+v",
				unregistered.Context)
		})
}

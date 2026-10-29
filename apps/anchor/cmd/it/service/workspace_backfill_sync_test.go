package service_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/model"
	"anchor/internal/domain/resource"
	"anchor/internal/domain/resourcetype"
)

// A workspace a release before the workspace sync wrote has no resource, and
// its membership no assignment. The backfill registers them and replays its
// marks once its transaction commits, so the relationship store already holds
// the workspace under its Organization and the moved assignment, with no mark
// left for the sweeper.
func TestWorkspaceBackfillSync(t *testing.T) {
	openFGA(t)

	t.Run("The store holds a backfilled workspace's parent and its moved assignment", func(t *testing.T) {
		productID := GivenATenantAndProduct(t).Product.ID
		_, err := ResourceTypeService.Create(t.Context(), resourcetype.CreateInput{
			ProductID: productID, Slug: resourcetype.WorkspaceSlug, Name: resourcetype.WorkspaceName,
			ParentTypes: []string{resourcetype.OrganizationSlug},
		})
		require.NoError(t, err)
		w := registryWorld{productID: productID, organizationID: givenOrganization(t, productID)}
		member := givenProductUser(t, productID)
		_, err = TestDB.ExecContext(t.Context(), `INSERT INTO organization_memberships
			(organization_id, product_user_id, product_role_id) VALUES ($1, $2, $3)`,
			w.organizationID, member, givenScopedRole(t, w, ""))
		require.NoError(t, err)
		editor := givenScopedRole(t, w, resourcetype.WorkspaceSlug)
		workspaceID := ids.MustNew("ws")
		_, err = TestDB.ExecContext(t.Context(),
			`INSERT INTO workspaces (id, organization_id, name) VALUES ($1, $2, $3)`,
			workspaceID, w.organizationID, "Payments "+uuid.NewString())
		require.NoError(t, err)
		_, err = TestDB.ExecContext(t.Context(), `INSERT INTO workspace_memberships
			(workspace_id, product_user_id, product_role_id) VALUES ($1, $2, $3)`, workspaceID, member, editor)
		require.NoError(t, err)

		require.NoError(t, WorkspaceBackfill.RunProduct(t.Context(), productID))

		tree, err := ResourceTypeService.Tree(t.Context(), productID)
		require.NoError(t, err)
		found, err := ResourceRepo.Find(t.Context(), w.organizationID,
			tree.BySlug(resourcetype.WorkspaceSlug).OrElse(resourcetype.ResourceType{}).ID, workspaceID)
		require.NoError(t, err)
		registered, ok := found.Get()
		require.True(t, ok, "the workspace is registered")
		assert.Equal(t, resource.OrganizationRef(w.organizationID), registered.Parent)
		assert.Contains(t, tuplesOf(t, productID, registered), fga.Tuple{
			Object: model.ResourceObject(registered.Type, registered.RowID), Relation: model.RelationParent,
			User: model.Organization(w.organizationID),
		})
		assert.ElementsMatch(t, heldTuples(registered, editor, member),
			assignmentWorld{registryWorld: w}.assignmentTuples(t, registered, editor))
		assert.Empty(t, w.pendingResources(t))
	})
}

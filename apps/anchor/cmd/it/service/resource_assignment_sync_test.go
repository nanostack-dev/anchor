package service_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/fgasync"
	"anchor/internal/authz/model"
	"anchor/internal/domain/organization"
	"anchor/internal/domain/resource"
	"anchor/internal/domain/roleassignment"
)

// assignmentWorld is a registry world with a project holding a folder, two
// folder roles of the Organization, and a member.
type assignmentWorld struct {
	registryWorld
	project, folder resource.Resource
	editor, viewer  string
	member          string
}

func givenAssignmentWorld(t *testing.T) assignmentWorld {
	t.Helper()
	w := givenRegistryWorld(t)
	project, _ := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
	folder, _ := w.putCollected(t, "folder", uuid.NewString(), project.Ref())
	w.syncResources(t, project, folder)
	userID := givenProductUser(t, w.productID)
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO organization_memberships
		(organization_id, product_user_id, product_role_id) VALUES ($1, $2, $3)`,
		w.organizationID, userID, givenScopedRole(t, w, ""))
	require.NoError(t, err)
	return assignmentWorld{
		registryWorld: w, project: project, folder: folder, member: userID,
		editor: givenScopedRole(t, w, "folder"), viewer: givenScopedRole(t, w, "folder"),
	}
}

// givenScopedRole inserts a role the Product manages, scoped to the type, or
// to the Organization when slug is empty.
func givenScopedRole(t *testing.T, w registryWorld, slug string) string {
	t.Helper()
	roleID := "role_" + uuid.NewString()
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO product_roles
		(id, product_id, type, name, resource_type_id)
		VALUES ($1, $2::text, 'MANAGED', $1,
			(SELECT id FROM product_resource_types WHERE product_id = $2::text AND slug = COALESCE(NULLIF($3::text, ''), 'organization')))`,
		roleID, w.productID, slug)
	require.NoError(t, err)
	return roleID
}

// assign puts the member's role on the resource the way a request does, and
// answers the objects its write marked.
func (w assignmentWorld) assign(t *testing.T, r resource.Resource, userID, roleID string) []string {
	t.Helper()
	ctx, marked := fgasync.CollectMarks(t.Context())
	_, err := RoleAssignmentService.Put(ctx, roleassignment.PutInput{
		Target:  w.target(r, userID),
		RoleIDs: []string{roleID},
	})
	require.NoError(t, err)
	return resourceKeys(marked())
}

func (w assignmentWorld) target(r resource.Resource, userID string) roleassignment.Target {
	return roleassignment.Target{
		ProductID: w.productID, OrganizationID: w.organizationID, ResourceType: r.Type, ResourceID: r.ExternalID,
		ProductUserID: userID,
	}
}

func (w assignmentWorld) collect(t *testing.T, write func(ctx context.Context) error) []string {
	t.Helper()
	ctx, marked := fgasync.CollectMarks(t.Context())
	require.NoError(t, write(ctx))
	return resourceKeys(marked())
}

func tuplesOf(t *testing.T, productID string, r resource.Resource) []fga.Tuple {
	t.Helper()
	return tuplesOn(t, productID, model.ResourceObject(r.Type, r.RowID))
}

// assignmentTuples reads the tuples the store holds on the resource's object
// and on every assignment object of the resource's row, linked or not.
func (w assignmentWorld) assignmentTuples(t *testing.T, r resource.Resource, roles ...string) []fga.Tuple {
	t.Helper()
	var tuples []fga.Tuple
	objects := []string{model.ResourceObject(r.Type, r.RowID)}
	for _, roleID := range roles {
		objects = append(objects, model.Assignment(r.RowID, roleID))
	}
	for _, object := range objects {
		for _, tuple := range tuplesOn(t, w.productID, object) {
			if tuple.Relation != model.RelationParent {
				tuples = append(tuples, tuple)
			}
		}
	}
	return tuples
}

func heldTuples(r resource.Resource, roleID string, userIDs ...string) []fga.Tuple {
	assignment := model.Assignment(r.RowID, roleID)
	tuples := []fga.Tuple{
		{Object: model.ResourceObject(r.Type, r.RowID), Relation: model.RelationAssignment, User: assignment},
		{Object: assignment, Relation: model.RelationRole, User: model.Role(roleID)},
	}
	for _, userID := range userIDs {
		tuples = append(
			tuples,
			fga.Tuple{Object: assignment, Relation: model.RelationAssignee, User: model.User(userID)},
		)
	}
	return tuples
}

func TestResourceAssignmentSync(t *testing.T) {
	openFGA(t)

	t.Run("An assignment marks its resource only and its sync writes the assignment tuples", func(t *testing.T) {
		w := givenAssignmentWorld(t)

		marked := w.assign(t, w.folder, w.member, w.editor)
		w.syncResources(t, w.folder)

		assert.Equal(t, objectsOf(w.folder), marked)
		assert.ElementsMatch(t, heldTuples(w.folder, w.editor, w.member), w.assignmentTuples(t, w.folder, w.editor))
		assert.Contains(t, tuplesOf(t, w.productID, w.folder), fga.Tuple{
			Object: model.ResourceObject(w.folder.Type, w.folder.RowID), Relation: model.RelationParent,
			User: model.ResourceObject(w.project.Type, w.project.RowID),
		}, "the parent tuple stays")
		assert.Empty(t, w.pendingResources(t))
	})

	t.Run("Two members holding one role share its assignment", func(t *testing.T) {
		w := givenAssignmentWorld(t)
		other := givenProductUser(t, w.productID)
		_, err := TestDB.ExecContext(t.Context(), `INSERT INTO organization_memberships
			(organization_id, product_user_id, product_role_id) VALUES ($1, $2, $3)`,
			w.organizationID, other, w.viewer)
		require.NoError(t, err)

		w.assign(t, w.folder, w.member, w.editor)
		w.assign(t, w.folder, other, w.editor)
		w.syncResources(t, w.folder)

		assert.ElementsMatch(
			t,
			heldTuples(w.folder, w.editor, w.member, other),
			w.assignmentTuples(t, w.folder, w.editor),
		)
	})

	t.Run("A replaced role moves the member to the new role's assignment and drops the old one", func(t *testing.T) {
		w := givenAssignmentWorld(t)
		w.assign(t, w.folder, w.member, w.editor)
		w.syncResources(t, w.folder)

		marked := w.assign(t, w.folder, w.member, w.viewer)
		w.syncResources(t, w.folder)

		assert.Equal(t, objectsOf(w.folder), marked)
		assert.ElementsMatch(t, heldTuples(w.folder, w.viewer, w.member),
			w.assignmentTuples(t, w.folder, w.editor, w.viewer))
	})

	t.Run("A removed assignment drops its tuples", func(t *testing.T) {
		w := givenAssignmentWorld(t)
		w.assign(t, w.folder, w.member, w.editor)
		w.syncResources(t, w.folder)

		marked := w.collect(t, func(ctx context.Context) error {
			return RoleAssignmentService.Delete(ctx, w.target(w.folder, w.member))
		})
		w.syncResources(t, w.folder)

		assert.Equal(t, objectsOf(w.folder), marked)
		assert.Empty(t, w.assignmentTuples(t, w.folder, w.editor))
	})

	t.Run("A member who leaves the organization loses every resource assignment's tuples", func(t *testing.T) {
		w := givenAssignmentWorld(t)
		projectRole := givenScopedRole(t, w.registryWorld, "project")
		w.assign(t, w.folder, w.member, w.editor)
		w.assign(t, w.project, w.member, projectRole)
		w.syncResources(t, w.project, w.folder)

		marked := w.collect(t, func(ctx context.Context) error {
			return MembershipService.RemoveMember(ctx, organization.RemoveMemberInput{
				ProductID: w.productID, OrganizationID: w.organizationID, ProductUserID: w.member,
			})
		})
		w.syncResources(t, w.project, w.folder)

		assert.ElementsMatch(t, objectsOf(w.folder, w.project), marked)
		assert.Empty(t, w.assignmentTuples(t, w.folder, w.editor))
		assert.Empty(t, w.assignmentTuples(t, w.project, projectRole))
	})

	t.Run("A cascade delete drops the assignment tuples of every deleted resource", func(t *testing.T) {
		w := givenAssignmentWorld(t)
		projectRole := givenScopedRole(t, w.registryWorld, "project")
		w.assign(t, w.folder, w.member, w.editor)
		w.assign(t, w.project, w.member, projectRole)
		w.syncResources(t, w.project, w.folder)

		marked := w.deleteCollected(t, w.project, true)
		w.syncResources(t, w.project, w.folder)

		assert.ElementsMatch(t, objectsOf(w.folder, w.project), marked, "each deleted resource is marked once")
		assert.Empty(t, w.assignmentTuples(t, w.folder, w.editor))
		assert.Empty(t, w.assignmentTuples(t, w.project, projectRole))
		assert.Empty(t, parentsOf(t, w.productID, w.folder))
	})

	t.Run("A stray assignee on a resource's assignment is deleted by its next sync", func(t *testing.T) {
		w := givenAssignmentWorld(t)
		w.assign(t, w.folder, w.member, w.editor)
		w.syncResources(t, w.folder)
		stray := fga.Tuple{
			Object: model.Assignment(w.folder.RowID, w.editor), Relation: model.RelationAssignee,
			User: model.User(givenProductUser(t, w.productID)),
		}
		require.NoError(t, productStore(t, w.productID).Apply(t.Context(), fga.Changes{Writes: []fga.Tuple{stray}}))

		w.syncResources(t, w.folder)

		assert.ElementsMatch(t, heldTuples(w.folder, w.editor, w.member), w.assignmentTuples(t, w.folder, w.editor))
	})

	t.Run("An assignment's replay reads no other resource and the assignments sync writes none", func(t *testing.T) {
		w := givenAssignmentWorld(t)
		other, _ := w.putCollected(t, "folder", uuid.NewString(), w.project.Ref())

		marked := w.assign(t, w.folder, w.member, w.editor)
		syncKey(t, w.productID, assignments, w.organizationID)
		assert.Empty(t, w.assignmentTuples(t, w.folder, w.editor), "the assignments sync wrote no assignment")
		w.syncResources(t, w.folder)

		assert.False(t, slices.Contains(marked, model.ResourceObject(other.Type, other.RowID)))
		assert.Empty(t, parentsOf(t, w.productID, other), "the other folder was never synced")
	})

	t.Run("The reconcile writes the assignment tuples no replay wrote", func(t *testing.T) {
		w := givenAssignmentWorld(t)
		w.assign(t, w.folder, w.member, w.editor)

		require.NoError(t, AuthzSyncer.Reconcile(t.Context(), w.productID))

		assert.ElementsMatch(t, heldTuples(w.folder, w.editor, w.member), w.assignmentTuples(t, w.folder, w.editor))
	})
}

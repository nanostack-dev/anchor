package service_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accessservice "anchor/internal/access/service"
	"anchor/internal/authz/model"
	"anchor/internal/domain/resource"
	"anchor/internal/domain/roleassignment"
)

// deepChainWorld is a registry world with the deepest chain the registry
// accepts: the Organization, a project, then folders down to level 15.
type deepChainWorld struct {
	registryWorld
	project, deepest resource.Resource
	permission       string
}

func givenDeepChainWorld(t *testing.T) deepChainWorld {
	t.Helper()
	w := givenRegistryWorld(t)
	project, _ := w.putCollected(t, "project", uuid.NewString(), resource.OrganizationRef(w.organizationID))
	chain := []resource.Resource{project}
	for len(chain)+1 < resource.MaxDepth {
		folder, _ := w.putCollected(t, "folder", uuid.NewString(), chain[len(chain)-1].Ref())
		chain = append(chain, folder)
	}
	w.syncResources(t, chain...)
	permission := "folder:" + uuid.NewString()
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO product_resource_permissions
		(product_id, name, resource_type_id)
		VALUES ($1::text, $2, (SELECT id FROM product_resource_types WHERE product_id = $1::text AND slug = 'folder'))`,
		w.productID, permission)
	require.NoError(t, err)
	return deepChainWorld{registryWorld: w, project: project, deepest: chain[len(chain)-1], permission: permission}
}

// member adds a member holding the role in the Organization, with an external
// ID the evaluator can name.
func (w deepChainWorld) member(t *testing.T, roleID string) (string, string) {
	t.Helper()
	userID := givenProductUser(t, w.productID)
	externalID := "user_" + userID
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO organization_memberships
		(organization_id, product_user_id, product_role_id) VALUES ($1, $2, $3)`, w.organizationID, userID, roleID)
	require.NoError(t, err)
	_, err = TestDB.ExecContext(t.Context(),
		`UPDATE product_users SET external_id = $1 WHERE id = $2`, externalID, userID)
	require.NoError(t, err)
	return userID, externalID
}

func (w deepChainWorld) ask(externalID string) accessservice.Request {
	return accessservice.Ask(w.productID).User(externalID).Can(w.permission)
}

func TestAccessDecisionsOnResources(t *testing.T) {
	openFGA(t)

	t.Run("An organization role reaches the bottom of the deepest chain and one level below it", func(t *testing.T) {
		w := givenDeepChainWorld(t)
		reader := givenScopedRole(t, w.registryWorld, "")
		givenRolePermission(t, w.productID, reader, w.permission)
		_, externalID := w.member(t, reader)
		require.NoError(t, AuthzSyncer.Reconcile(t.Context(), w.productID))

		registered := evaluate(t, w.ask(externalID).OnResource(w.organizationID, "folder", w.deepest.ExternalID))
		unregistered := evaluate(t, w.ask(externalID).OnResource(w.organizationID, "folder", uuid.NewString()).
			Under("folder", w.deepest.ExternalID).Explaining())

		assert.True(t, registered.Allowed)
		assert.True(t, unregistered.Allowed, "the contextual parent adds a sixteenth level")
		require.Len(t, unregistered.Items, 1)
		assert.Equal(t, []accessservice.Grant{{
			RoleID: reader, RoleName: roleName(t, reader),
			Resource: accessservice.GrantResource{Type: "organization", ID: w.organizationID},
		}}, unregistered.Items[0].GrantedBy)
		assert.Empty(t, tuplesOn(t, w.productID, model.UnregisteredResource("folder")),
			"the contextual parent is never stored")
	})

	t.Run("A role on the project reaches an unregistered folder below the deepest one", func(t *testing.T) {
		w := givenDeepChainWorld(t)
		member := givenScopedRole(t, w.registryWorld, "")
		lead := givenScopedRole(t, w.registryWorld, "project")
		givenRolePermission(t, w.productID, lead, w.permission)
		userID, externalID := w.member(t, member)
		_, other := w.member(t, member)
		_, err := RoleAssignmentService.Put(t.Context(), roleassignment.PutInput{
			ProductID: w.productID, OrganizationID: w.organizationID,
			ResourceType: w.project.Type, ResourceID: w.project.ExternalID, ProductUserID: userID,
			RoleIDs: []string{lead},
		})
		require.NoError(t, err)
		require.NoError(t, AuthzSyncer.Reconcile(t.Context(), w.productID))
		below := func(externalID string) accessservice.Request {
			return w.ask(externalID).OnResource(w.organizationID, "folder", uuid.NewString()).
				Under("folder", w.deepest.ExternalID)
		}

		allowed := evaluate(t, below(externalID))
		denied := evaluate(t, below(other))

		assert.True(t, allowed.Allowed)
		assert.False(t, denied.Allowed)
		assert.Equal(t, accessservice.ReasonMissingPermission, denied.Items[0].Reason)
	})
}

func roleName(t *testing.T, roleID string) string {
	t.Helper()
	var name string
	require.NoError(t, TestDB.QueryRowContext(t.Context(),
		`SELECT name FROM product_roles WHERE id = $1`, roleID).Scan(&name))
	return name
}

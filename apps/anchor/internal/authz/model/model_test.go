package model_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/model"
)

var relationPattern = regexp.MustCompile(`^[a-z0-9_]{1,50}$`)

func TestRelationNames(t *testing.T) {
	t.Parallel()

	t.Run("a permission with characters OpenFGA refuses becomes a safe relation", func(t *testing.T) {
		t.Parallel()
		for _, permission := range []string{"flows:read", "api_key:*", "Status-Pages.Publish", "a"} {
			relation := model.PermissionRelation(permission)
			assert.Regexp(t, relationPattern, relation, permission)
			assert.True(t, strings.HasPrefix(relation, "can_"), relation)
		}
	})

	t.Run("names that read alike still get different relations", func(t *testing.T) {
		t.Parallel()
		assert.NotEqual(t, model.PermissionRelation("flows:read"), model.PermissionRelation("flows_read"))
		assert.NotEqual(t, model.RoleRelation("x"), model.PermissionRelation("x"))
	})

	t.Run("a long role ID is cut to OpenFGA's limit and stays unique", func(t *testing.T) {
		t.Parallel()
		long := "product_role_" + strings.Repeat("a", 80)
		relation := model.RoleRelation(long)
		assert.Regexp(t, relationPattern, relation)
		assert.NotEqual(t, relation, model.RoleRelation(long+"b"))
	})
}

func TestBuild(t *testing.T) {
	t.Parallel()

	catalog := model.Catalog{
		Roles: []model.Role{
			{ID: "role_viewer", Permissions: []string{"flows:read"}},
			{ID: "role_admin", Permissions: []string{"flows:read", "flows:delete"}},
		},
		Permissions: []string{"flows:read", "flows:delete", "billing:read"},
	}

	t.Run("the model parses and grants a permission through every role that holds it", func(t *testing.T) {
		t.Parallel()
		built, err := model.Build(catalog)
		require.NoError(t, err)
		require.NotNil(t, built.Proto)
		read := model.PermissionRelation("flows:read")
		assert.Contains(t, built.DSL, "define "+read+": [api_key] or "+
			model.RoleRelation("role_admin")+" or "+model.RoleRelation("role_viewer"))
		assert.Contains(t, built.DSL, "define "+read+": "+
			model.RoleRelation("role_admin")+" or "+model.RoleRelation("role_viewer")+" or "+read+" from organization")
	})

	t.Run("a permission no role grants is held on a workspace only through the organization", func(t *testing.T) {
		t.Parallel()
		built, err := model.Build(catalog)
		require.NoError(t, err)
		billing := model.PermissionRelation("billing:read")
		assert.Contains(t, built.DSL, "define "+billing+": [api_key]\n")
		assert.Contains(t, built.DSL, "define "+billing+": "+billing+" from organization\n")
	})

	t.Run("the same catalog in another order builds the same model", func(t *testing.T) {
		t.Parallel()
		shuffled := model.Catalog{
			Roles: []model.Role{
				{ID: "role_admin", Permissions: []string{"flows:delete", "flows:read"}},
				{ID: "role_viewer", Permissions: []string{"flows:read"}},
			},
			Permissions: []string{"billing:read", "flows:delete", "flows:read"},
		}
		first, err := model.Build(catalog)
		require.NoError(t, err)
		second, err := model.Build(shuffled)
		require.NoError(t, err)
		assert.Equal(t, first.DSL, second.DSL)
	})

	t.Run("an empty catalog still builds a model", func(t *testing.T) {
		t.Parallel()
		_, err := model.Build(model.Catalog{})
		require.NoError(t, err)
	})
}

func TestOrganizationStateTuples(t *testing.T) {
	t.Parallel()

	catalog := model.Catalog{
		Roles:       []model.Role{{ID: "role_member", Permissions: []string{"flows:read"}}},
		Permissions: []string{"flows:read"},
	}
	state := model.OrganizationState{
		ProductID:      "prd_1",
		OrganizationID: "org_1",
		Exists:         true,
		Members:        []model.RoleAssignment{{ProductUserID: "usr_alice", RoleID: "role_member"}},
		Workspaces: []model.WorkspaceState{{ID: "ws_1", Members: []model.RoleAssignment{
			{ProductUserID: "usr_alice", RoleID: "role_member"},
			{ProductUserID: "usr_gone", RoleID: "role_member"},
			{ProductUserID: "usr_alice", RoleID: "role_deleted"},
		}}},
		APIKeys: []model.APIKeyGrant{{ID: "key_1", Permissions: []string{"flows:read", "removed:permission"}}},
	}

	t.Run("an organization implies its product, members, workspaces and key grants", func(t *testing.T) {
		t.Parallel()
		role := model.RoleRelation("role_member")
		assert.ElementsMatch(t, []model.Tuple{
			{Object: "organization:org_1", Relation: model.RelationProduct, User: "product:prd_1"},
			{Object: "organization:org_1", Relation: model.RelationMember, User: "user:usr_alice"},
			{Object: "organization:org_1", Relation: role, User: "user:usr_alice"},
			{Object: "workspace:ws_1", Relation: model.RelationOrganization, User: "organization:org_1"},
			{Object: "workspace:ws_1", Relation: role, User: "user:usr_alice"},
			{Object: "organization:org_1", Relation: model.PermissionRelation("flows:read"), User: "api_key:key_1"},
		}, state.Tuples(catalog))
	})

	t.Run("a deleted organization implies nothing", func(t *testing.T) {
		t.Parallel()
		deleted := state
		deleted.Exists = false
		assert.Empty(t, deleted.Tuples(catalog))
	})
}

func TestDiff(t *testing.T) {
	t.Parallel()

	keep := model.Tuple{Object: "organization:o", Relation: "member", User: "user:a"}
	stale := model.Tuple{Object: "organization:o", Relation: "member", User: "user:b"}
	missing := model.Tuple{Object: "organization:o", Relation: "member", User: "user:c"}

	t.Run("only what differs is written or deleted, once", func(t *testing.T) {
		t.Parallel()
		changes := model.Diff([]model.Tuple{keep, stale, stale}, []model.Tuple{keep, missing, missing})
		assert.Equal(t, []model.Tuple{missing}, changes.Writes)
		assert.Equal(t, []model.Tuple{stale}, changes.Deletes)
	})

	t.Run("equal sides need no change", func(t *testing.T) {
		t.Parallel()
		assert.True(t, model.Diff([]model.Tuple{keep}, []model.Tuple{keep}).IsEmpty())
	})
}

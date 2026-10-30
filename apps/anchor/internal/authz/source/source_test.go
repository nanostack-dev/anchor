package source_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/fga/fgatest"
	"anchor/internal/authz/fgasync"
	"anchor/internal/authz/model"
	"anchor/internal/authz/source"
	"anchor/internal/events"
)

func sources() []fgasync.Source {
	return []fgasync.Source{
		source.Assignments{}, source.RoleGrants{}, source.APIKeyGrants{}, source.ResourceParents{},
		source.ResourceAssignments{},
	}
}

// eventWithEveryField carries every data field, so a source that reads
// one finds it.
func eventWithEveryField(eventType events.Type) events.Event {
	return events.Event{Type: eventType, ProductID: "prod_tasks", Data: events.Data{
		events.FieldOrganizationID: "org_acme", events.FieldProductUserID: "pu_ana",
		events.FieldInvitationID: "inv_1", events.FieldAPIKeyID: "key_ci",
		events.FieldRoleID: "role_admin", events.FieldPermissionName: "billing:write",
		events.FieldResourceTypeID: "rt_1", events.FieldResourceType: "project", events.FieldResourceRowID: "res_1",
	}}
}

func claimed(t *testing.T, eventType events.Type) bool {
	t.Helper()
	event := eventWithEveryField(eventType)
	if source.ChangesModel(event) {
		return true
	}
	for _, p := range sources() {
		keys, err := p.Stale(event)
		require.NoError(t, err)
		if len(keys) > 0 {
			return true
		}
	}
	return false
}

func TestEveryEventIsClassified(t *testing.T) {
	t.Run("Each event type is claimed by a source or the catalog, or listed as without access", func(t *testing.T) {
		for _, eventType := range declaredEventTypes(t) {
			listed := slices.Contains(source.WithoutAccess(), eventType)
			isClaimed := claimed(t, eventType)

			assert.NotEqual(t, listed, isClaimed,
				"%s: claim it in a source's Stale or the catalog, or list it in WithoutAccess, not both", eventType)
		}
	})
}

func TestStale(t *testing.T) {
	t.Run("A membership change makes its Organization's assignments stale", func(t *testing.T) {
		keys, err := source.Assignments{}.Stale(eventWithEveryField(events.MembershipCreated))

		require.NoError(t, err)
		assert.Equal(t, []string{"org_acme"}, keys)
	})

	t.Run("A deleted managed role makes every Organization's assignments stale", func(t *testing.T) {
		keys, err := source.Assignments{}.Stale(eventWithEveryField(events.ProductRoleDeleted))

		require.NoError(t, err)
		assert.Equal(t, []string{fgasync.EveryKey}, keys)
	})

	t.Run("A role change makes that role's grants stale", func(t *testing.T) {
		keys, err := source.RoleGrants{}.Stale(eventWithEveryField(events.OrganizationRoleUpdated))

		require.NoError(t, err)
		assert.Equal(t, []string{"role_admin"}, keys)
	})

	t.Run("A role event without its role fails the write", func(t *testing.T) {
		_, err := source.RoleGrants{}.Stale(events.Event{Type: events.ProductRoleUpdated, ProductID: "prod_tasks"})

		require.Error(t, err)
	})

	t.Run("A resource write makes its own object stale", func(t *testing.T) {
		keys, err := source.ResourceParents{}.Stale(eventWithEveryField(events.ResourceUpdated))

		require.NoError(t, err)
		assert.Equal(t, []string{model.ResourceObject("project", "res_1")}, keys)
	})

	t.Run("A role assignment on a resource makes that resource's assignments stale", func(t *testing.T) {
		keys, err := source.ResourceAssignments{}.Stale(eventWithEveryField(events.ResourceRoleAssignmentDeleted))

		require.NoError(t, err)
		assert.Equal(t, []string{model.ResourceObject("project", "res_1")}, keys)
	})

	t.Run("A resource event without its row fails the write", func(t *testing.T) {
		_, err := source.ResourceParents{}.Stale(events.Event{
			Type:      events.ResourceCreated,
			ProductID: "prod_tasks",
			Data:      events.Data{events.FieldResourceType: "project"},
		})

		require.Error(t, err)
	})

	t.Run("A permission change rewrites the model", func(t *testing.T) {
		assert.True(t, source.ChangesModel(eventWithEveryField(events.ProductResourcePermissionDeleted)))
		assert.False(t, source.ChangesModel(eventWithEveryField(events.ProductRoleUpdated)))
	})
}

func TestTuples(t *testing.T) {
	t.Run("An Organization links one assignment per role, however many members hold it", func(t *testing.T) {
		tuples := source.AssignmentTuples("org_acme",
			source.Holder{ProductUserID: "pu_ana", RoleRowID: "role_admin"},
			source.Holder{ProductUserID: "pu_bob", RoleRowID: "role_admin"},
		)

		assignment := model.Assignment("org_acme", "role_admin")
		assert.ElementsMatch(t, []fga.Tuple{
			{Object: "organization:org_acme", Relation: model.RelationAssignment, User: assignment},
			{Object: assignment, Relation: model.RelationRole, User: "role:role_admin"},
			{Object: assignment, Relation: model.RelationAssignee, User: "user:pu_ana"},
			{Object: assignment, Relation: model.RelationAssignee, User: "user:pu_bob"},
		}, tuples)
	})

	t.Run("An assignment's ID gives back its role tuple", func(t *testing.T) {
		assignment := model.Assignment("org_acme", "role_admin")

		roleTuple, ok := source.AssignmentRoleTuple(assignment).Get()

		require.True(t, ok)
		assert.Equal(t, fga.Tuple{Object: assignment, Relation: model.RelationRole, User: "role:role_admin"}, roleTuple)
		assert.False(t, source.AssignmentRoleTuple("role_assignment:broken").IsPresent())
	})

	t.Run("A resource sits under its parent, or under its Organization at the top", func(t *testing.T) {
		assert.Equal(t,
			fga.Tuple{Object: "rt_project:res_1", Relation: model.RelationParent, User: "organization:org_acme"},
			source.ResourceParentTuple("org_acme", "project", "res_1", "", ""))
		assert.Equal(t,
			fga.Tuple{Object: "rt_folder:res_2", Relation: model.RelationParent, User: "rt_project:res_1"},
			source.ResourceParentTuple("org_acme", "folder", "res_2", "project", "res_1"))
	})

	t.Run("A resource links one assignment per role held on it, named by its row", func(t *testing.T) {
		tuples := source.ResourceAssignmentTuples("project", "res_1",
			source.Holder{ProductUserID: "pu_ana", RoleRowID: "role_editor"},
			source.Holder{ProductUserID: "pu_bob", RoleRowID: "role_editor"},
		)

		assignment := model.Assignment("res_1", "role_editor")
		assert.ElementsMatch(t, []fga.Tuple{
			{Object: "rt_project:res_1", Relation: model.RelationAssignment, User: assignment},
			{Object: assignment, Relation: model.RelationRole, User: "role:role_editor"},
			{Object: assignment, Relation: model.RelationAssignee, User: "user:pu_ana"},
			{Object: assignment, Relation: model.RelationAssignee, User: "user:pu_bob"},
		}, tuples)
	})

	t.Run("A role grants each of its permissions to every user", func(t *testing.T) {
		assert.Equal(t, []fga.Tuple{
			{Object: "role:role_admin", Relation: model.GrantsRelation("billing:write"), User: model.AllUsers},
		}, source.RoleGrantTuples("role_admin", []string{"billing:write"}))
	})
}

func TestOwnership(t *testing.T) {
	t.Run("The sources own disjoint tuples of one Organization and its resource", func(t *testing.T) {
		compiled, err := model.Compile(model.Catalog{
			Types: []model.Type{
				{Slug: model.TypeOrganization},
				{Slug: "project", ParentTypes: []string{model.TypeOrganization}},
			},
			Permissions: []model.Permission{{Name: "billing:write", Scope: model.TypeOrganization}},
		})
		require.NoError(t, err)
		engine := fgatest.NewEngine(t)
		storeID := fgatest.NewStore(t, engine)
		modelID, err := fga.WriteModel(t.Context(), engine, storeID, compiled.Proto)
		require.NoError(t, err)
		store := fga.NewStore(engine, storeID, modelID)
		assignments := source.AssignmentTuples("org_acme",
			source.Holder{ProductUserID: "pu_ana", RoleRowID: "role_admin"})
		grants := source.RoleGrantTuples("role_admin", []string{"billing:write"})
		keys := []fga.Tuple{{
			Object: "organization:org_acme", Relation: model.CanRelation("billing:write"), User: "api_key:key_ci",
		}}
		parents := []fga.Tuple{source.ResourceParentTuple("org_acme", "project", "res_1", "", "")}
		onResource := source.ResourceAssignmentTuples("project", "res_1",
			source.Holder{ProductUserID: "pu_ana", RoleRowID: "role_admin"})
		require.NoError(t, store.Apply(t.Context(), fga.Changes{
			Writes: slices.Concat(assignments, grants, keys, parents, onResource),
		}))

		storedAssignments, err := source.Assignments{}.Stored(t.Context(), store, "org_acme")
		require.NoError(t, err)
		storedGrants, err := source.RoleGrants{}.Stored(t.Context(), store, "role_admin")
		require.NoError(t, err)
		storedKeys, err := source.APIKeyGrants{}.Stored(t.Context(), store, "org_acme")
		require.NoError(t, err)

		assert.ElementsMatch(t, assignments, storedAssignments)
		assert.ElementsMatch(t, grants, storedGrants)
		storedParents, err := source.ResourceParents{}.Stored(t.Context(), store, "rt_project:res_1")
		require.NoError(t, err)

		assert.ElementsMatch(t, keys, storedKeys)
		storedOnResource, err := source.ResourceAssignments{}.Stored(t.Context(), store, "rt_project:res_1")
		require.NoError(t, err)
		assert.ElementsMatch(t, parents, storedParents)
		assert.ElementsMatch(t, onResource, storedOnResource)
	})
}

// declaredEventTypes lists the events.Type constants in internal/events.
func declaredEventTypes(t *testing.T) []events.Type {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "events", "types.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	require.NoError(t, err)
	var declared []events.Type
	for _, decl := range file.Decls {
		gen, isGen := decl.(*ast.GenDecl)
		if !isGen || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, isValue := spec.(*ast.ValueSpec)
			if !isValue || value.Type == nil || len(value.Values) == 0 {
				continue
			}
			if ident, isIdent := value.Type.(*ast.Ident); !isIdent || ident.Name != "Type" {
				continue
			}
			lit, isLit := value.Values[0].(*ast.BasicLit)
			require.True(t, isLit)
			unquoted, unquoteErr := strconv.Unquote(lit.Value)
			require.NoError(t, unquoteErr)
			declared = append(declared, events.Type(unquoted))
		}
	}
	require.NotEmpty(t, declared)
	return declared
}

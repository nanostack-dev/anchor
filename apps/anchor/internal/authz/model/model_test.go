package model_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/openfga/openfga/pkg/typesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/model"
)

func documentCatalog() model.Catalog {
	return model.Catalog{
		Types: []model.Type{
			{Slug: "organization"},
			{Slug: "project", ParentTypes: []string{"organization"}},
			{Slug: "folder", ParentTypes: []string{"project"}, NestsInItself: true},
		},
		Permissions: []model.Permission{
			{Name: "billing:read", Scope: "organization"},
			{Name: "document:view", Scope: "folder"},
		},
	}
}

func TestCompile(t *testing.T) {
	t.Run("A permission is held on its scope and every type above it, never below", func(t *testing.T) {
		compiled, err := model.Compile(documentCatalog())

		require.NoError(t, err)
		folder := section(compiled.DSL, "type rt_folder")
		organization := section(compiled.DSL, "type organization")
		assert.Contains(t, folder, model.CanRelation("document:view"))
		assert.NotContains(t, folder, model.CanRelation("billing:read"))
		assert.Contains(t, organization, model.CanRelation("billing:read"))
		assert.Contains(t, organization, model.CanRelation("document:view"))
	})

	t.Run("Only organization lets an API key hold a permission directly", func(t *testing.T) {
		compiled, err := model.Compile(documentCatalog())

		require.NoError(t, err)
		assert.Contains(t, section(compiled.DSL, "type organization"), "[api_key]")
		assert.NotContains(t, section(compiled.DSL, "type rt_project"), "[api_key]")
	})

	t.Run("A self-nesting type is its own parent", func(t *testing.T) {
		compiled, err := model.Compile(documentCatalog())

		require.NoError(t, err)
		assert.Contains(t, section(compiled.DSL, "type rt_folder"), "define parent: [rt_folder, rt_project]")
	})

	t.Run("The same catalog in any order compiles the same model", func(t *testing.T) {
		catalog := documentCatalog()
		reversed := model.Catalog{
			Types:       []model.Type{catalog.Types[2], catalog.Types[1], catalog.Types[0]},
			Permissions: []model.Permission{catalog.Permissions[1], catalog.Permissions[0]},
		}

		first, err := model.Compile(catalog)
		require.NoError(t, err)
		second, err := model.Compile(reversed)
		require.NoError(t, err)

		assert.Equal(t, first.Hash, second.Hash)
	})

	t.Run("Any valid slug or permission name compiles", func(t *testing.T) {
		slugs := []string{"define", "and", "or", "from", "condition", "relations", "with", "in", "self", "this",
			"user", "rt_user", "role", "api_key", "role_assignment"}
		catalog := model.Catalog{Types: []model.Type{{Slug: "organization"}}}
		for _, slug := range slugs {
			catalog.Types = append(catalog.Types, model.Type{Slug: slug, ParentTypes: []string{"organization"}})
			catalog.Permissions = append(catalog.Permissions, model.Permission{Name: slug + ":view", Scope: slug})
		}
		catalog.Permissions = append(catalog.Permissions,
			model.Permission{Name: "two\nlines\n  define x: [user]", Scope: "organization"})

		compiled, err := model.Compile(catalog)

		require.NoError(t, err)
		_, err = typesystem.NewAndValidate(t.Context(), compiled.Proto)
		require.NoError(t, err)
	})

	t.Run("The catalog hash ignores order and follows every change", func(t *testing.T) {
		catalog := documentCatalog()
		changed := documentCatalog()
		changed.Types[2].NestsInItself = false

		assert.Equal(t, model.Hash(catalog), model.Hash(model.Catalog{
			Types:       []model.Type{catalog.Types[1], catalog.Types[0], catalog.Types[2]},
			Permissions: []model.Permission{catalog.Permissions[1], catalog.Permissions[0]},
		}))
		assert.NotEqual(t, model.Hash(catalog), model.Hash(changed))
	})
}

func TestRelationNames(t *testing.T) {
	t.Run("Two permissions that sanitize alike keep distinct relations within the length limit", func(t *testing.T) {
		colon := model.CanRelation("doc:read")
		underscore := model.CanRelation("doc_read")
		long := model.CanRelation(strings.Repeat("x", 120))

		assert.NotEqual(t, colon, underscore)
		assert.LessOrEqual(t, len(long), 50)
	})
}

func TestOrganizationTuples(t *testing.T) {
	t.Run("Members are assignees of their role's assignment, and the role grants its permissions", func(t *testing.T) {
		state := model.OrganizationState{
			OrganizationID: "org_1", Exists: true,
			Memberships: []model.Membership{
				{ProductUserID: "pusr_1", RoleRowID: "role_1"},
				{ProductUserID: "pusr_2", RoleRowID: "role_1"},
			},
			Roles:   []model.RoleGrants{{RowID: "role_1", Permissions: []string{"document:view", "removed:perm"}}},
			APIKeys: []model.APIKeyGrants{{ID: "key_1", Permissions: []string{"billing:read"}}},
		}

		tuples := state.Tuples(documentCatalog())

		assignment := model.Assignment("org_1", "role_1")
		assert.ElementsMatch(t, []model.Tuple{
			{Object: assignment, Relation: model.RelationRole, User: model.Role("role_1")},
			{Object: assignment, Relation: model.RelationAssignee, User: model.User("pusr_1")},
			{Object: assignment, Relation: model.RelationAssignee, User: model.User("pusr_2")},
			{Object: model.Organization("org_1"), Relation: model.RelationAssignment, User: assignment},
			{Object: model.Role("role_1"), Relation: model.GrantsRelation("document:view"), User: model.AllUsers},
			{
				Object:   model.Organization("org_1"),
				Relation: model.CanRelation("billing:read"),
				User:     model.APIKey("key_1"),
			},
		}, tuples)
	})

	t.Run("The organization links one assignment per role, however many members hold it", func(t *testing.T) {
		state := model.OrganizationState{OrganizationID: "org_1", Exists: true}
		for i := range 1000 {
			state.Memberships = append(state.Memberships,
				model.Membership{ProductUserID: "pusr_" + strconv.Itoa(i), RoleRowID: "role_" + strconv.Itoa(i%2)})
		}

		links := 0
		for _, tuple := range state.Tuples(documentCatalog()) {
			if tuple.Object == model.Organization("org_1") && tuple.Relation == model.RelationAssignment {
				links++
			}
		}

		assert.Equal(t, 2, links)
	})

	t.Run("A deleted organization implies no tuple", func(t *testing.T) {
		assert.Empty(t, model.OrganizationState{OrganizationID: "org_1"}.Tuples(documentCatalog()))
	})

	t.Run("An assignment's ID gives back its role tuple", func(t *testing.T) {
		assignment := model.Assignment("org_1", "role_1")

		assert.Equal(t, []model.Tuple{
			{Object: assignment, Relation: model.RelationRole, User: model.Role("role_1")},
		}, model.AssignmentTuples(assignment))
	})

	t.Run("Diff writes what is missing and deletes what is left over", func(t *testing.T) {
		keep := model.Tuple{Object: "a", Relation: "r", User: "u"}
		drop := model.Tuple{Object: "b", Relation: "r", User: "u"}
		add := model.Tuple{Object: "c", Relation: "r", User: "u"}

		changes := model.Diff([]model.Tuple{keep, drop, drop}, []model.Tuple{keep, add})

		assert.Equal(t, []model.Tuple{add}, changes.Writes)
		assert.Equal(t, []model.Tuple{drop}, changes.Deletes)
	})
}

// section returns the DSL of one type, up to the next type.
func section(dsl, header string) string {
	start := strings.Index(dsl, header+"\n")
	if start < 0 {
		return ""
	}
	rest := dsl[start+len(header):]
	if before, _, ok := strings.Cut(rest, "\ntype "); ok {
		return before
	}
	return rest
}

func TestResourceTuples(t *testing.T) {
	t.Run("A registered resource holds one parent tuple to its parent or its organization", func(t *testing.T) {
		states := []model.ResourceState{
			{Object: "rt_project:res_1", Exists: true, Resource: model.Resource{
				OrganizationID: "org_1", RowID: "res_1", Type: "project",
			}},
			{Object: "rt_folder:res_2", Exists: true, Resource: model.Resource{
				OrganizationID: "org_1", RowID: "res_2", Type: "folder", ParentRowID: "res_1", ParentType: "project",
			}},
			{Object: "rt_folder:res_3", Exists: true, Resource: model.Resource{
				OrganizationID: "org_1", RowID: "res_3", Type: "folder", ParentRowID: "res_2", ParentType: "folder",
			}},
		}

		var tuples []model.Tuple
		for _, state := range states {
			tuples = append(tuples, state.Tuples(documentCatalog())...)
		}

		assert.Equal(t, []model.Tuple{
			{Object: "rt_project:res_1", Relation: model.RelationParent, User: model.Organization("org_1")},
			{Object: "rt_folder:res_2", Relation: model.RelationParent, User: "rt_project:res_1"},
			{Object: "rt_folder:res_3", Relation: model.RelationParent, User: "rt_folder:res_2"},
		}, tuples)
	})

	t.Run("A parent tuple the model's parent relation would refuse is left out", func(t *testing.T) {
		for _, r := range []model.Resource{
			{OrganizationID: "org_1", RowID: "res_1", Type: "folder"},
			{OrganizationID: "org_1", RowID: "res_2", Type: "project", ParentRowID: "res_3", ParentType: "project"},
			{OrganizationID: "org_1", RowID: "res_4", Type: "removed", ParentRowID: "res_1", ParentType: "folder"},
		} {
			assert.Empty(
				t,
				model.ResourceState{Object: r.Object(), Exists: true, Resource: r}.Tuples(documentCatalog()),
			)
		}
	})

	t.Run("A resource whose row is gone implies no tuple", func(t *testing.T) {
		assert.Empty(t, model.ResourceState{Object: "rt_project:res_1"}.Tuples(documentCatalog()))
	})

	t.Run("A resource object parses back into its type and row", func(t *testing.T) {
		slug, rowID, ok := model.ParseResourceObject(model.ResourceObject("folder", "res_1"))

		assert.True(t, ok)
		assert.Equal(t, "folder", slug)
		assert.Equal(t, "res_1", rowID)
		_, _, ok = model.ParseResourceObject(model.Organization("org_1"))
		assert.False(t, ok)
	})
}

func TestAssignmentIDs(t *testing.T) {
	t.Run("An organization's assignment keeps its ID", func(t *testing.T) {
		assert.Equal(t, "role_assignment:org_1.role_1", model.Assignment("org_1", "role_1"))
	})

	t.Run("A resource's assignment is named by the resource row and the role row", func(t *testing.T) {
		assignment := model.Assignment("res_1", "role_1")

		parts, ok := model.ParseAssignment(assignment)

		require.True(t, ok)
		assert.Equal(t, "role_assignment:res_1.role_1", assignment)
		assert.Equal(t, model.AssignmentParts{ObjectID: "res_1", RoleRowID: "role_1"}, parts)
		assert.NotEqual(t, model.Assignment("org_1", "role_1"), assignment)
	})

	t.Run("A reference that is not an assignment does not parse", func(t *testing.T) {
		for _, object := range []string{"role:role_1", "role_assignment:res_1", "role_assignment:.role_1",
			"role_assignment:res_1.", "role_assignment:res_1.role.1"} {
			_, ok := model.ParseAssignment(object)
			assert.False(t, ok, object)
		}
	})
}

func TestResourceAssignmentTuples(t *testing.T) {
	folder := model.Resource{
		OrganizationID: "org_1", RowID: "res_2", Type: "folder", ParentRowID: "res_1", ParentType: "project",
	}

	t.Run("Each role held on a resource is one linked assignment with every member as an assignee", func(t *testing.T) {
		state := model.ResourceState{
			Object:   folder.Object(),
			Exists:   true,
			Resource: folder,
			Assignments: []model.ResourceAssignment{
				{ProductUserID: "pusr_1", RoleRowID: "role_1"},
				{ProductUserID: "pusr_2", RoleRowID: "role_1"},
				{ProductUserID: "pusr_3", RoleRowID: "role_2"},
			},
		}

		tuples := state.Tuples(documentCatalog())

		editors, viewers := model.Assignment("res_2", "role_1"), model.Assignment("res_2", "role_2")
		assert.ElementsMatch(t, []model.Tuple{
			{Object: "rt_folder:res_2", Relation: model.RelationParent, User: "rt_project:res_1"},
			{Object: "rt_folder:res_2", Relation: model.RelationAssignment, User: editors},
			{Object: editors, Relation: model.RelationRole, User: model.Role("role_1")},
			{Object: editors, Relation: model.RelationAssignee, User: model.User("pusr_1")},
			{Object: editors, Relation: model.RelationAssignee, User: model.User("pusr_2")},
			{Object: "rt_folder:res_2", Relation: model.RelationAssignment, User: viewers},
			{Object: viewers, Relation: model.RelationRole, User: model.Role("role_2")},
			{Object: viewers, Relation: model.RelationAssignee, User: model.User("pusr_3")},
		}, tuples)
		assert.Equal(t, []string{editors, viewers}, model.AssignmentObjects(tuples))
	})

	t.Run("A resource whose row is gone implies none of its assignments", func(t *testing.T) {
		state := model.ResourceState{Object: folder.Object(), Assignments: []model.ResourceAssignment{
			{ProductUserID: "pusr_1", RoleRowID: "role_1"},
		}}

		assert.Empty(t, state.Tuples(documentCatalog()))
	})

	t.Run("A type the model lacks links no assignment", func(t *testing.T) {
		removed := model.Resource{OrganizationID: "org_1", RowID: "res_4", Type: "removed"}
		state := model.ResourceState{Object: removed.Object(), Exists: true, Resource: removed,
			Assignments: []model.ResourceAssignment{{ProductUserID: "pusr_1", RoleRowID: "role_1"}}}

		assert.Empty(t, state.Tuples(documentCatalog()))
	})
}

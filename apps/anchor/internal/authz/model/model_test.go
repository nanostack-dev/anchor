package model_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openfga/language/pkg/go/transformer"
	"github.com/openfga/openfga/pkg/typesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

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
	t.Run("A permission is held on its scope and every type above it", func(t *testing.T) {
		compiled := model.Compile(documentCatalog())

		folder := section(compiled.DSL, "type rt_folder")
		organization := section(compiled.DSL, "type organization")
		view := model.CanRelation("document:view")
		assert.Contains(t, folder, view+": "+view+" from assignment or "+view+" from parent")
		assert.Contains(t, organization, model.CanRelation("billing:read"))
		assert.Contains(t, organization, view)
	})

	t.Run("A type below a permission's scope answers it through its parent only", func(t *testing.T) {
		compiled := model.Compile(documentCatalog())

		billing := model.CanRelation("billing:read")
		assert.Contains(t, section(compiled.DSL, "type rt_folder"), billing+": "+billing+" from parent\n")
		assert.Contains(t, section(compiled.DSL, "type rt_project"), billing+": "+billing+" from parent\n")
	})

	t.Run("A type answers no permission scoped to a type off its chain", func(t *testing.T) {
		catalog := documentCatalog()
		catalog.Types = append(catalog.Types, model.Type{Slug: "invoice", ParentTypes: []string{"organization"}})
		catalog.Permissions = append(catalog.Permissions, model.Permission{Name: "invoice:pay", Scope: "invoice"})

		compiled := model.Compile(catalog)

		assert.NotContains(t, section(compiled.DSL, "type rt_folder"), model.CanRelation("invoice:pay"))
		assert.Contains(t, section(compiled.DSL, "type rt_invoice"), model.CanRelation("billing:read"))
	})

	t.Run("A type with parents on two branches answers what either branch scopes", func(t *testing.T) {
		catalog := model.Catalog{
			Types: []model.Type{
				{Slug: "organization"},
				{Slug: "team", ParentTypes: []string{"organization"}},
				{Slug: "project", ParentTypes: []string{"organization"}},
				{Slug: "board", ParentTypes: []string{"team", "project"}},
			},
			Permissions: []model.Permission{
				{Name: "team:lead", Scope: "team"},
				{Name: "project:manage", Scope: "project"},
			},
		}

		compiled := model.Compile(catalog)

		board := section(compiled.DSL, "type rt_board")
		assert.Contains(t, board, model.CanRelation("team:lead"))
		assert.Contains(t, board, model.CanRelation("project:manage"))
		_, err := typesystem.NewAndValidate(t.Context(), compiled.Proto)
		require.NoError(t, err)
	})

	t.Run("Only organization lets an API key hold a permission directly", func(t *testing.T) {
		compiled := model.Compile(documentCatalog())

		assert.Contains(t, section(compiled.DSL, "type organization"), "[api_key]")
		assert.NotContains(t, section(compiled.DSL, "type rt_project"), "[api_key]")
	})

	t.Run("A self-nesting type is its own parent", func(t *testing.T) {
		compiled := model.Compile(documentCatalog())

		assert.Contains(t, section(compiled.DSL, "type rt_folder"), "define parent: [rt_folder, rt_project]")
	})

	t.Run("The same catalog in any order compiles the same model", func(t *testing.T) {
		catalog := documentCatalog()
		reversed := model.Catalog{
			Types:       []model.Type{catalog.Types[2], catalog.Types[1], catalog.Types[0]},
			Permissions: []model.Permission{catalog.Permissions[1], catalog.Permissions[0]},
		}

		first := model.Compile(catalog)
		second := model.Compile(reversed)

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

		compiled := model.Compile(catalog)

		_, err := typesystem.NewAndValidate(t.Context(), compiled.Proto)
		require.NoError(t, err)
	})

	t.Run("The proto is the model OpenFGA parses from the DSL", func(t *testing.T) {
		keywords := model.Catalog{Types: []model.Type{{Slug: "organization"}}}
		for _, slug := range []string{"define", "and", "or", "from", "self", "user", "role"} {
			keywords.Types = append(keywords.Types, model.Type{Slug: slug, ParentTypes: []string{"organization"}})
			keywords.Permissions = append(keywords.Permissions, model.Permission{Name: slug + ":view", Scope: slug})
		}
		large := model.Catalog{Types: []model.Type{{Slug: "organization"}}}
		for i := range 30 {
			large.Types = append(large.Types, model.Type{
				Slug: fmt.Sprintf("type_%02d", i), ParentTypes: []string{"organization"}, NestsInItself: i%3 == 0,
			})
			large.Permissions = append(large.Permissions,
				model.Permission{Name: fmt.Sprintf("doc:perm_%02d", i), Scope: fmt.Sprintf("type_%02d", i)},
				model.Permission{Name: fmt.Sprintf("org:perm_%02d", i), Scope: "organization"})
		}
		catalogs := map[string]model.Catalog{
			"documents":         documentCatalog(),
			"organization only": {Types: []model.Type{{Slug: "organization"}}},
			"several parents":   {Types: []model.Type{{Slug: "organization"}, {Slug: "a", ParentTypes: []string{"organization"}}, {Slug: "b", ParentTypes: []string{"organization", "a"}}}, Permissions: []model.Permission{{Name: "b:read", Scope: "b"}, {Name: "a:read", Scope: "a"}}},
			"OpenFGA keywords":  keywords,
			"thirty types":      large,
		}
		for name, catalog := range catalogs {
			compiled := model.Compile(catalog)

			parsed, err := transformer.TransformDSLToProto(compiled.DSL)

			require.NoError(t, err, name)
			assert.True(t, proto.Equal(parsed, compiled.Proto), "%s:\nparsed   %s\ncompiled %s",
				name, protojson.Format(parsed), protojson.Format(compiled.Proto))
		}
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

func TestResourceObjects(t *testing.T) {
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

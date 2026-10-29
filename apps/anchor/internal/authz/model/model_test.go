package model_test

import (
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

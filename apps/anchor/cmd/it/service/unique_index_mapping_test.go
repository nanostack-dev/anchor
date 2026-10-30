package service_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// uniqueGuardsWithoutMapping lists the unique guards no write maps to a domain
// error, each with the reason no request can reach the violation.
var uniqueGuardsWithoutMapping = map[string]string{
	"idx_platform_users_tenant_email": "registration inserts users first in the same transaction, " +
		"so a second registration of an email stops on users_email_key",
	"platform_users_external_id_key": "registration copies users.external_id, which nothing writes",
	"users_external_id_key":          "nothing writes users.external_id",
	"platform_invitations_code_key":  "the code is 32 random bytes",
	"resources_organization_type_external_id_key": "a resource PUT reads the resource under the Organization's " +
		"resource lock, held exclusively, before it creates one",
	"resource_role_assignments_pkey": "an assignment PUT reads the member's roles on the resource under its " +
		"per-member advisory lock, and inserts only the roles they don't hold",
	"product_api_key_permissions_pkey": "permission names are deduplicated; a create inserts under a new key " +
		"and a replace runs after the key row's UPDATE, which serializes writers",
	"product_api_key_permissions_api_key_id_product_id_permissio_key": "same rows as product_api_key_permissions_pkey",
	"organization_api_key_permissions_pkey": "permission names are deduplicated and only a create, " +
		"under a new key, inserts them",
	"organization_api_key_permissi_api_key_id_organization_id_pe_key": "same rows as " +
		"organization_api_key_permissions_pkey",
	"product_role_resource_permiss_product_id_product_role_id_pe_key": "permission names are deduplicated; " +
		"a create inserts under a new role and assign and replace use ON CONFLICT",
	"email_template_versions_template_id_version_number_key": "versions are written under lockTemplate, " +
		"or with a brand-new template",
	"idx_email_template_versions_one_draft":     "versions are written under lockTemplate, or with a brand-new template",
	"idx_email_template_versions_one_published": "versions are written under lockTemplate",
	"license_schema_fields_license_schema_id_name_key": "declareFields rejects a repeated name, " +
		"and every write holds the license write lock",
	// Index names survive ALTER TABLE ... RENAME: this is product_organization_api_key_configs.
	"product_api_key_configs_pkey":        "upserted with ON CONFLICT (product_id)",
	"product_event_endpoint_configs_pkey": "upserted with ON CONFLICT (product_id)",
	"product_resource_type_parents_pkey": "INSERT ... SELECT yields each parent once; " +
		"an update replaces edges after the type row's UPDATE",
	"authz_stores_pkey":         "inserted with ON CONFLICT (product_id) DO NOTHING",
	"authz_stores_store_id_key": "OpenFGA generates every store ID",
	"authz_sync_marks_pkey":     "upserted with ON CONFLICT (product_id, source, scope_key)",
}

// retiredUniqueGuards are dropped by a migration but still mapped, because the
// release before the drop runs against the schema that has them.
var retiredUniqueGuards = []string{
	"product_roles_product_id_name_key",           // 000064
	"idx_product_roles_product_lower_name_unique", // 000065
}

// uniqueGuardsQuery lists the unique guards in the migrated schema. A guard over
// an id column is left out: the server generates every id. So are
// schema_migrations and pgkit's pgqueue tables, which this code does not write.
const uniqueGuardsQuery = `
SELECT i.relname
FROM pg_index x
JOIN pg_class i ON i.oid = x.indexrelid
JOIN pg_class t ON t.oid = x.indrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
WHERE n.nspname = 'public' AND x.indisunique
  AND t.relname <> 'schema_migrations' AND t.relname NOT LIKE 'pgqueue%'
  AND NOT EXISTS (
    SELECT 1 FROM pg_attribute a
    WHERE a.attrelid = x.indrelid AND a.attname = 'id' AND a.attnum = ANY (x.indkey))
ORDER BY 1`

// A pre-check cannot see a concurrent writer's uncommitted row, so a guard no
// write maps to a domain error answers the loser of that race with a 500.
func TestEveryUniqueGuardMapsToADomainError(t *testing.T) {
	rows, err := TestDB.QueryContext(t.Context(), uniqueGuardsQuery)
	require.NoError(t, err)
	defer rows.Close()
	var guards []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		guards = append(guards, name)
	}
	require.NoError(t, rows.Err())
	require.NotEmpty(t, guards)

	mapped := mappedUniqueGuards(t)
	var unmapped []string
	for _, guard := range guards {
		if !slices.Contains(mapped, guard) && uniqueGuardsWithoutMapping[guard] == "" {
			unmapped = append(unmapped, guard)
		}
	}
	assert.Empty(t, unmapped,
		"map each with OnUnique on the write that can trip it (docs/development/agent-rules.md), "+
			"or list it in uniqueGuardsWithoutMapping with the reason no request reaches the violation")

	// A misspelt name never matches, so the mapping or the exemption is dead.
	for _, name := range slices.Concat(mapped, slices.Collect(maps.Keys(uniqueGuardsWithoutMapping))) {
		if !slices.Contains(retiredUniqueGuards, name) {
			assert.Contains(t, guards, name, "no unique guard in the schema has this name")
		}
	}
	for _, name := range retiredUniqueGuards {
		assert.NotContains(t, guards, name, "still in the schema, so not retired")
	}
}

// mappedUniqueGuards returns the constraint names passed to OnUnique and
// pgerr.IsUniqueViolation under internal/.
func mappedUniqueGuards(t *testing.T) []string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "internal")

	fset := token.NewFileSet()
	filesByDir := map[string][]*ast.File{}
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		filesByDir[filepath.Dir(path)] = append(filesByDir[filepath.Dir(path)], file)
		return nil
	}))

	var names []string
	for _, files := range filesByDir {
		consts := stringConsts(files)
		for _, file := range files {
			ast.Inspect(file, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall || len(call.Args) < 2 {
					return true
				}
				sel, isSel := call.Fun.(*ast.SelectorExpr)
				if !isSel || (sel.Sel.Name != "OnUnique" && sel.Sel.Name != "IsUniqueViolation") {
					return true
				}
				for _, arg := range call.Args[1:] { // the first is the sentinel or the error
					name, resolved := stringValue(arg, consts)
					require.True(t, resolved, "%s: name the guard with a string constant of this package",
						fset.Position(arg.Pos()))
					names = append(names, name)
				}
				return true
			})
		}
	}
	return names
}

func stringConsts(files []*ast.File) map[string]string {
	consts := map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, isGen := decl.(*ast.GenDecl)
			if !isGen || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, isValue := spec.(*ast.ValueSpec)
				if !isValue {
					continue
				}
				for i, ident := range value.Names {
					if i >= len(value.Values) {
						continue
					}
					if s, isString := stringValue(value.Values[i], nil); isString {
						consts[ident.Name] = s
					}
				}
			}
		}
	}
	return consts
}

func stringValue(expr ast.Expr, consts map[string]string) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			s, err := strconv.Unquote(e.Value)
			return s, err == nil
		}
	case *ast.Ident:
		s, ok := consts[e.Name]
		return s, ok
	}
	return "", false
}

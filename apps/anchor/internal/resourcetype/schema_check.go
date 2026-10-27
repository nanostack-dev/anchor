package resourcetype

import (
	"context"
	"database/sql"
	"fmt"

	"go.uber.org/fx"
)

// Querier runs the schema check's queries; *sql.DB and *sql.Conn are both one.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// RegisterSchemaCheck refuses to start a release whose migrations were
// forced past instead of applied: autoFixDirty marks a failed migration as
// done, and the code would then run on a schema it doesn't expect.
func RegisterSchemaCheck(lc fx.Lifecycle, db *sql.DB) {
	lc.Append(fx.Hook{OnStart: func(ctx context.Context) error { return CheckSchema(ctx, db) }})
}

// The role migrations, by version.
const (
	migrationRoleColumns          = 59
	migrationRoleOrganizationKey  = 60
	migrationOptionalMemberRole   = 61
	migrationManagedRoleNames     = 62
	migrationCustomRoleNames      = 63
	migrationDropProductNameKey   = 64
	migrationDropProductNameIndex = 65
	migrationRoleTypeRequired     = 66
)

// roleSchemaFact is one thing the role migrations (000059 to 000066) must
// leave true. holds is a query that returns one boolean. Without a recovery,
// the recovery is to force the version before and redeploy.
type roleSchemaFact struct {
	holds    string
	args     []any
	problem  string
	version  int
	recovery string
}

func validIndex(name string, version int) roleSchemaFact {
	return roleSchemaFact{
		holds: `
SELECT COALESCE(bool_and(i.indisvalid AND i.indisready), false) FROM pg_class c
JOIN pg_index i ON i.indexrelid = c.oid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relname = $1 AND n.nspname = current_schema()`,
		args:    []any{name},
		problem: "index " + name + " is missing or invalid",
		version: version,
		recovery: fmt.Sprintf("run `DROP INDEX CONCURRENTLY %s`, then `migrate force %d`, then redeploy",
			name, version-1),
	}
}

// roleSchemaFacts lists them in migration order. autoFixDirty marks a failed
// migration as applied, and a CONCURRENTLY build that failed leaves an
// INVALID index, so neither shows until something reads the schema.
func roleSchemaFacts() []roleSchemaFact {
	return []roleSchemaFact{
		{
			holds: `
SELECT count(*) = 2 FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'product_roles'
  AND column_name IN ('organization_id', 'type')`,
			problem: "product_roles has no organization_id or no type column", version: migrationRoleColumns,
		},
		{
			holds: `
SELECT COALESCE(bool_and(c.convalidated), false) FROM pg_constraint c
JOIN pg_namespace n ON n.oid = c.connamespace
WHERE c.conname = 'product_roles_organization_id_fkey' AND n.nspname = current_schema()`,
			problem: "product_roles_organization_id_fkey is missing or not validated",
			version: migrationRoleOrganizationKey,
		},
		{
			holds: `
SELECT count(*) = 2 FROM information_schema.columns
WHERE table_schema = current_schema() AND column_name = 'product_role_id'
  AND table_name IN ('organization_memberships', 'organization_invitations') AND is_nullable = 'YES'`,
			problem: "the membership or invitation role is still NOT NULL", version: migrationOptionalMemberRole,
		},
		validIndex("idx_product_roles_managed_lower_name", migrationManagedRoleNames),
		validIndex("idx_product_roles_custom_lower_name", migrationCustomRoleNames),
		{
			holds: `
SELECT NOT EXISTS (SELECT 1 FROM pg_constraint c JOIN pg_namespace n ON n.oid = c.connamespace
WHERE c.conname = 'product_roles_product_id_name_key' AND n.nspname = current_schema())`,
			problem: "the product-wide role name constraint product_roles_product_id_name_key still exists",
			version: migrationDropProductNameKey,
		},
		{
			holds: `
SELECT NOT EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relname = 'idx_product_roles_product_lower_name_unique' AND n.nspname = current_schema())`,
			problem: "the product-wide role name index idx_product_roles_product_lower_name_unique still exists",
			version: migrationDropProductNameIndex,
		},
		{
			holds: `
SELECT count(*) = 1 FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'product_roles'
  AND column_name = 'type' AND is_nullable = 'NO'`,
			problem: "product_roles.type is still nullable", version: migrationRoleTypeRequired,
		},
	}
}

// CheckSchema reports the first column that migration 000058 should have made
// NOT NULL and didn't, then the first role schema fact that does not hold.
func CheckSchema(ctx context.Context, db Querier) error {
	if err := checkNotNull(ctx, db); err != nil {
		return err
	}
	for _, fact := range roleSchemaFacts() {
		if err := checkFact(ctx, db, fact); err != nil {
			return err
		}
	}
	return nil
}

func checkFact(ctx context.Context, db Querier, fact roleSchemaFact) error {
	rows, err := db.QueryContext(ctx, fact.holds, fact.args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	holds := false
	if rows.Next() {
		if err = rows.Scan(&holds); err != nil {
			return err
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if !holds {
		recovery := fact.recovery
		if recovery == "" {
			recovery = fmt.Sprintf("run `migrate force %d`, then redeploy", fact.version-1)
		}
		return fmt.Errorf("%s: migration %06d was forced, not applied; %s", fact.problem, fact.version, recovery)
	}
	return nil
}

func checkNotNull(ctx context.Context, db Querier) error {
	rows, err := db.QueryContext(ctx, `
SELECT table_name FROM information_schema.columns
WHERE table_schema = current_schema() AND column_name = 'resource_type_id'
  AND table_name IN ('product_roles', 'product_resource_permissions') AND is_nullable = 'YES'
ORDER BY table_name`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			return err
		}
		return fmt.Errorf(
			"%s.resource_type_id is nullable: migration 000058 was forced, not applied; "+
				"run `migrate force 57`, fix the data, then redeploy", table)
	}
	return rows.Err()
}

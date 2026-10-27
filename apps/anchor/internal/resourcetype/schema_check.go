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

// CheckSchema reports the first column that migration 000058 should have made
// NOT NULL and didn't.
func CheckSchema(ctx context.Context, db Querier) error {
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

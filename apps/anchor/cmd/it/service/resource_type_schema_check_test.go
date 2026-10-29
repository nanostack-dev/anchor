package service_test

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/resourcetype"
)

// autoFixDirty marks a failed migration as applied, so the release checks the
// schema it needs before it starts.
func TestResourceTypeSchemaCheck(t *testing.T) {
	t.Run("The migrated schema passes the check", func(t *testing.T) {
		require.NoError(t, resourcetype.CheckSchema(t.Context(), TestDB))
	})

	t.Run("A nullable resource_type_id fails the check and names the recovery", func(t *testing.T) {
		tx, err := TestDB.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		schema := "schema_check_" + uuid.NewString()[:8]
		for _, stmt := range []string{
			`CREATE SCHEMA ` + schema,
			`CREATE TABLE ` + schema + `.product_roles (resource_type_id VARCHAR(255))`,
			`CREATE TABLE ` + schema + `.product_resource_permissions (resource_type_id VARCHAR(255) NOT NULL)`,
			`SET LOCAL search_path TO ` + schema,
		} {
			_, err = tx.ExecContext(t.Context(), stmt)
			require.NoError(t, err)
		}

		err = resourcetype.CheckSchema(t.Context(), tx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "product_roles.resource_type_id is nullable")
		assert.Contains(t, err.Error(), "migrate force 57")
	})

	for _, broken := range []struct {
		name     string
		breakIt  string
		problem  string
		recovery string
	}{
		{
			name:     "Missing role columns fail the check and name the recovery",
			breakIt:  `ALTER TABLE product_roles DROP COLUMN type`,
			problem:  "product_roles has no organization_id or no type column",
			recovery: "migrate force 58",
		},
		{
			name: "An unvalidated organization foreign key fails the check and names the recovery",
			breakIt: `UPDATE pg_constraint SET convalidated = false WHERE conname = 'product_roles_organization_id_fkey'
				AND connamespace = current_schema()::regnamespace`,
			problem:  "product_roles_organization_id_fkey is missing or not validated",
			recovery: "migrate force 59",
		},
		{
			name:     "A membership role still NOT NULL fails the check and names the recovery",
			breakIt:  `ALTER TABLE organization_memberships ALTER COLUMN product_role_id SET NOT NULL`,
			problem:  "the membership or invitation role is still NOT NULL",
			recovery: "migrate force 60",
		},
		{
			name:     "A missing role name guard fails the check and names the recovery",
			breakIt:  `DROP INDEX idx_product_roles_managed_lower_name`,
			problem:  "index idx_product_roles_managed_lower_name is missing or invalid",
			recovery: "`DROP INDEX CONCURRENTLY idx_product_roles_managed_lower_name`, then `migrate force 61`",
		},
		{
			name: "An invalid role name guard fails the check and names the recovery",
			breakIt: `UPDATE pg_index SET indisvalid = false
				WHERE indexrelid = 'idx_product_roles_custom_lower_name'::regclass`,
			problem:  "index idx_product_roles_custom_lower_name is missing or invalid",
			recovery: "migrate force 62",
		},
		{
			name:     "A product-wide role name constraint left behind fails the check and names the recovery",
			breakIt:  `ALTER TABLE product_roles ADD CONSTRAINT product_roles_product_id_name_key UNIQUE (product_id, name)`,
			problem:  "product_roles_product_id_name_key still exists",
			recovery: "migrate force 63",
		},
		{
			name: "A product-wide role name index left behind fails the check and names the recovery",
			breakIt: `CREATE UNIQUE INDEX idx_product_roles_product_lower_name_unique
				ON product_roles (product_id, lower(name))`,
			problem:  "idx_product_roles_product_lower_name_unique still exists",
			recovery: "migrate force 64",
		},
		{
			name:     "A nullable role type fails the check and names the recovery",
			breakIt:  `ALTER TABLE product_roles ALTER COLUMN type DROP NOT NULL`,
			problem:  "product_roles.type is still nullable",
			recovery: "migrate force 65",
		},
		{
			name:     "A missing custom roles switch fails the check and names the recovery",
			breakIt:  `ALTER TABLE product_resource_types DROP COLUMN allows_custom_roles`,
			problem:  "product_resource_types has no allows_custom_roles column",
			recovery: "migrate force 66",
		},
		{
			name:     "Missing sync marks fail the check and name the recovery",
			breakIt:  `DROP TABLE authz_sync_marks`,
			problem:  "authz_sync_marks or the authz_stores sync columns are missing",
			recovery: "migrate force 68",
		},
	} {
		t.Run(broken.name, func(t *testing.T) {
			tx := migratedSchemaTx(t)
			_, err := tx.ExecContext(t.Context(), broken.breakIt)
			require.NoError(t, err)

			err = resourcetype.CheckSchema(t.Context(), tx)

			require.Error(t, err)
			assert.Contains(t, err.Error(), broken.problem)
			assert.Contains(t, err.Error(), broken.recovery)
		})
	}

	t.Run("A schema of its own built like the migrations passes the check", func(t *testing.T) {
		require.NoError(t, resourcetype.CheckSchema(t.Context(), migratedSchemaTx(t)))
	})
}

// migratedSchemaTx opens a transaction, rolled back at the end of the test, on
// a schema of its own that holds what the role migrations leave behind.
func migratedSchemaTx(t *testing.T) *sql.Tx {
	t.Helper()
	tx, err := TestDB.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	schema := "schema_check_" + uuid.NewString()[:8]
	for _, stmt := range []string{
		`CREATE SCHEMA ` + schema,
		`SET LOCAL search_path TO ` + schema,
		`CREATE TABLE organizations (id VARCHAR(255) PRIMARY KEY)`,
		`CREATE TABLE product_roles (id VARCHAR(255) PRIMARY KEY, resource_type_id VARCHAR(255) NOT NULL,
			product_id VARCHAR(255), name VARCHAR(100), organization_id VARCHAR(255), type VARCHAR(10) NOT NULL)`,
		`ALTER TABLE product_roles ADD CONSTRAINT product_roles_organization_id_fkey
			FOREIGN KEY (organization_id) REFERENCES organizations(id)`,
		`CREATE TABLE product_resource_permissions (resource_type_id VARCHAR(255) NOT NULL)`,
		`CREATE TABLE product_resource_types (allows_custom_roles BOOLEAN)`,
		`CREATE TABLE organization_memberships (product_role_id VARCHAR(255))`,
		`CREATE TABLE organization_invitations (product_role_id VARCHAR(255))`,
		`CREATE TABLE authz_sync_marks (product_id VARCHAR(255))`,
		`CREATE TABLE authz_stores (model_id VARCHAR(64), model_hash VARCHAR(64), reconciled_at TIMESTAMPTZ)`,
		`CREATE UNIQUE INDEX idx_product_roles_managed_lower_name ON product_roles (product_id, lower(name))
			WHERE organization_id IS NULL`,
		`CREATE UNIQUE INDEX idx_product_roles_custom_lower_name ON product_roles (organization_id, lower(name))
			WHERE organization_id IS NOT NULL`,
	} {
		_, err = tx.ExecContext(t.Context(), stmt)
		require.NoError(t, err)
	}
	return tx
}

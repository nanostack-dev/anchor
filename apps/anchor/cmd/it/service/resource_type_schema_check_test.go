package service_test

import (
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
}

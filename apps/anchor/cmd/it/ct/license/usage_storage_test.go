package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two properties of the usage store that no API response can show, checked
// against the database directly.
func TestUsageObservationStorage(t *testing.T) {
	t.Run("observations live in a hypertable", func(t *testing.T) {
		// The aggregation, retention and compression a usage history needs are
		// TimescaleDB policies. A plain table would take all of them back into
		// hand-written Go. See docs/adr/0005-timescaledb-for-usage-history.md.
		var count int
		err := testDB.QueryRow(
			`SELECT count(*) FROM timescaledb_information.hypertables
			 WHERE hypertable_name = 'usage_observations'`,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "usage_observations is not a hypertable")
	})

	t.Run("deleting the product takes the observations with it", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.Usage().Report(gauge("flows", 37))

		// Cascading into a hypertable means cascading into its chunks. This
		// proves the delete completes rather than tripping the foreign key, and
		// losing the history with the product is deliberate.
		resp, err := w.client().DeleteProductWithResponse(context.Background(), w.productID())
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

		var count int
		err = testDB.QueryRow(
			`SELECT count(*) FROM usage_observations WHERE organization_id = $1`,
			w.OrganizationID(),
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("a later report appends rather than replacing", func(t *testing.T) {
		w := newLicenseWorld(t)

		w.Usage().Report(gauge("flows", 37))
		w.Usage().Report(gauge("flows", 41))

		// Both rows are kept. The second report does not overwrite the first,
		// which is the whole difference between a history and a current value
		// with a timestamp on it.
		var count int
		err := testDB.QueryRow(
			`SELECT count(*) FROM usage_observations WHERE organization_id = $1`,
			w.OrganizationID(),
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

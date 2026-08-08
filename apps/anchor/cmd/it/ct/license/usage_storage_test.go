package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageObservationStorage(t *testing.T) {
	t.Run("observations live in a hypertable", func(t *testing.T) {
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

		var count int
		err := testDB.QueryRow(
			`SELECT count(*) FROM usage_observations WHERE organization_id = $1`,
			w.OrganizationID(),
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

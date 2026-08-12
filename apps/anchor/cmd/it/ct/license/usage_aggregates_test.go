package license_ct_test

import (
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests prove the one sanctioned exception to "no business logic in
// SQL" — see docs/adr/0005-timescaledb-for-usage-history.md: bucketing and
// the minute-to-hour-to-day rollup cascade, for both gauges and windowed
// counters, and that retention's chunk drop is safe once an aggregate has
// captured a chunk.
//
// observed_at is stamped by Anchor at write time (docs/adr/0003), so the
// report endpoint cannot place two observations in different buckets on
// demand — every call happens at "now". These tests write straight to
// usage_observations instead, the only way to control it, and refresh the
// aggregates deliberately rather than waiting on the background policy.

// aggregateViews are the three levels of the cascade.
var aggregateViews = []string{
	"usage_observations_minute",
	"usage_observations_hour",
	"usage_observations_day",
}

// insertRawObservation writes directly to the hypertable, bypassing
// ReportUsage entirely so observed_at can be placed deliberately.
func insertRawObservation(
	t *testing.T,
	tenantID, productID, organizationID, key string,
	value float64, observedAt time.Time,
	from, to *time.Time,
) {
	t.Helper()
	_, err := testDB.Exec(
		`INSERT INTO usage_observations
			(id, platform_tenant_id, product_id, organization_id, key, value, window_from, window_to, observed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		ids.MustNew("uobs"), tenantID, productID, organizationID, key, value, from, to, observedAt,
	)
	require.NoError(t, err)
}

// refreshAggregates materializes every level of the cascade over its full
// range. Production relies on the refresh policy each migration adds; a test
// cannot wait on a background job running on its own schedule.
func refreshAggregates(t *testing.T) {
	t.Helper()
	for _, view := range aggregateViews {
		_, err := testDB.Exec(`CALL refresh_continuous_aggregate('` + view + `', NULL, NULL)`)
		require.NoError(t, err, view)
	}
}

// dropAllRawChunks force-drops every chunk of the raw hypertable, standing in
// for what the retention policy does gradually in production. The cutoff is
// in the future so even the chunk still open for writes qualifies.
//
// This is global to the hypertable, not scoped to one test's fixture, so it
// stays in its own test function and is never mixed into a shared world: every
// other test in this package writes and reads its own data within one test
// body, so execution order around this one cannot affect them.
func dropAllRawChunks(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(
		`SELECT drop_chunks('usage_observations', older_than => now() + interval '10 years')`,
	)
	require.NoError(t, err)
}

func TestUsageAggregateExistence(t *testing.T) {
	t.Run("each level of the cascade is a materialized-only continuous aggregate", func(t *testing.T) {
		// materialized_only rather than real-time aggregation: minute and hour are
		// themselves the source of the next level, which TimescaleDB requires, and
		// day matches them so every level's freshness is governed by the same
		// explicit refresh policy instead of mixing in a query-time computation.
		for _, view := range aggregateViews {
			var materializedOnly bool
			err := testDB.QueryRow(
				`SELECT materialized_only FROM timescaledb_information.continuous_aggregates
				 WHERE view_name = $1`,
				view,
			).Scan(&materializedOnly)
			require.NoError(t, err, view)
			assert.True(t, materializedOnly, view)
		}
	})
}

func TestUsageAggregateBucketing(t *testing.T) {
	t.Run("a minute bucket keeps the last gauge value observed within it", func(t *testing.T) {
		w := newLicenseWorld(t)
		minute := time.Date(2026, time.January, 5, 10, 30, 0, 0, time.UTC)

		insertRawObservation(t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_gauge", 10, minute, nil, nil)
		insertRawObservation(
			t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_gauge", 20,
			minute.Add(20*time.Second), nil, nil,
		)
		insertRawObservation(
			t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_gauge", 37,
			minute.Add(40*time.Second), nil, nil,
		)
		refreshAggregates(t)

		series := w.Usage().Series(seriesQuery(
			"cascade_gauge", ct.MINUTE, minute.Add(-time.Second), minute.Add(time.Minute),
		))

		require.Len(t, series.Items, 1)
		assert.WithinDuration(t, minute, series.Items[0].Bucket, time.Millisecond)
		assert.InDelta(t, 37.0, series.Items[0].Value, 0, "last(value, observed_at) must win, not sum or average")
	})

	t.Run("a minute bucket keeps the last windowed counter observed within it", func(t *testing.T) {
		w := newLicenseWorld(t)
		minute := time.Date(2026, time.January, 5, 11, 0, 0, 0, time.UTC)
		earlyFrom, earlyTo := billingPeriod()
		lateFrom, lateTo := earlyFrom.AddDate(0, 1, 0), earlyTo.AddDate(0, 1, 0)

		insertRawObservation(
			t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_counter", 100,
			minute, &earlyFrom, &earlyTo,
		)
		insertRawObservation(
			t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_counter", 412,
			minute.Add(30*time.Second), &lateFrom, &lateTo,
		)
		refreshAggregates(t)

		series := w.Usage().Series(seriesQuery(
			"cascade_counter", ct.MINUTE, minute.Add(-time.Second), minute.Add(time.Minute),
		))

		require.Len(t, series.Items, 1)
		assert.InDelta(t, 412.0, series.Items[0].Value, 0)
		require.NotNil(t, series.Items[0].From)
		require.NotNil(t, series.Items[0].To)
		assert.WithinDuration(t, lateFrom, *series.Items[0].From, time.Microsecond)
		assert.WithinDuration(t, lateTo, *series.Items[0].To, time.Microsecond)
	})
}

func TestUsageAggregateCascade(t *testing.T) {
	t.Run("hour and day roll up the last value across their span, for a gauge", func(t *testing.T) {
		w := newLicenseWorld(t)
		day := time.Date(2026, time.January, 6, 0, 0, 0, 0, time.UTC)

		// Three minutes in the same hour: the hour bucket must hold the last of
		// the three, never the first and never a sum.
		insertRawObservation(
			t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_gauge", 100,
			day.Add(10*time.Hour), nil, nil,
		)
		insertRawObservation(
			t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_gauge", 200,
			day.Add(10*time.Hour+15*time.Minute), nil, nil,
		)
		insertRawObservation(
			t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_gauge", 300,
			day.Add(10*time.Hour+59*time.Minute), nil, nil,
		)
		// A second hour, later the same day: the day bucket must hold this one,
		// not the 10:00 hour's, proving the cascade reads hour, not raw.
		insertRawObservation(
			t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_gauge", 400,
			day.Add(14*time.Hour), nil, nil,
		)
		refreshAggregates(t)

		hourSeries := w.Usage().Series(seriesQuery("cascade_gauge", ct.HOUR, day, day.AddDate(0, 0, 1)))
		require.Len(t, hourSeries.Items, 2)
		assert.InDelta(t, 300.0, hourSeries.Items[0].Value, 0, "10:00 hour must hold the last minute's value")
		assert.InDelta(t, 400.0, hourSeries.Items[1].Value, 0)

		daySeries := w.Usage().Series(seriesQuery("cascade_gauge", ct.DAY, day, day.AddDate(0, 0, 1)))
		require.Len(t, daySeries.Items, 1)
		assert.InDelta(t, 400.0, daySeries.Items[0].Value, 0, "day must hold the last hour's value, not the first")
	})

	t.Run("hour and day roll up the last value across their span, for a windowed counter", func(t *testing.T) {
		w := newLicenseWorld(t)
		day := time.Date(2026, time.January, 7, 0, 0, 0, 0, time.UTC)
		earlyFrom, earlyTo := billingPeriod()
		lateFrom, lateTo := earlyFrom.AddDate(0, 1, 0), earlyTo.AddDate(0, 1, 0)

		insertRawObservation(
			t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_counter", 1000,
			day.Add(9*time.Hour), &earlyFrom, &earlyTo,
		)
		insertRawObservation(
			t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_counter", 2000,
			day.Add(9*time.Hour+45*time.Minute), &lateFrom, &lateTo,
		)
		refreshAggregates(t)

		hourSeries := w.Usage().Series(seriesQuery("cascade_counter", ct.HOUR, day, day.AddDate(0, 0, 1)))
		require.Len(t, hourSeries.Items, 1)
		assert.InDelta(t, 2000.0, hourSeries.Items[0].Value, 0)
		require.NotNil(t, hourSeries.Items[0].From)
		assert.WithinDuration(t, lateFrom, *hourSeries.Items[0].From, time.Microsecond)

		daySeries := w.Usage().Series(seriesQuery("cascade_counter", ct.DAY, day, day.AddDate(0, 0, 1)))
		require.Len(t, daySeries.Items, 1)
		assert.InDelta(t, 2000.0, daySeries.Items[0].Value, 0)
		require.NotNil(t, daySeries.Items[0].To)
		assert.WithinDuration(t, lateTo, *daySeries.Items[0].To, time.Microsecond)
	})
}

func TestUsageAggregatePagination(t *testing.T) {
	t.Run("paginates minute buckets and orders them chronologically", func(t *testing.T) {
		w := newLicenseWorld(t)
		day := time.Date(2026, time.January, 8, 0, 0, 0, 0, time.UTC)

		for i := range 5 {
			insertRawObservation(
				t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_gauge",
				float64(i), day.Add(time.Duration(i)*time.Minute), nil, nil,
			)
		}
		refreshAggregates(t)

		query := seriesQuery("cascade_gauge", ct.MINUTE, day, day.Add(time.Hour))
		limit := int32(2)
		offset := int32(0)
		query.Limit = &limit
		query.Offset = &offset

		first := w.Usage().Series(query)
		assert.Equal(t, int64(5), first.Total)
		assert.Equal(t, 2, first.Count)
		require.Len(t, first.Items, 2)
		assert.InDelta(t, 0.0, first.Items[0].Value, 0)
		assert.InDelta(t, 1.0, first.Items[1].Value, 0)

		offset = 4
		last := w.Usage().Series(query)
		assert.Equal(t, int64(5), last.Total)
		require.Len(t, last.Items, 1)
		assert.InDelta(t, 4.0, last.Items[0].Value, 0)
	})
}

func TestUsageAggregateRetention(t *testing.T) {
	t.Run("dropping raw chunks leaves the coarser aggregates intact and queryable", func(t *testing.T) {
		w := newLicenseWorld(t)
		day := time.Date(2026, time.January, 9, 0, 0, 0, 0, time.UTC)

		insertRawObservation(
			t, w.tenantID, w.productID(), w.OrganizationID(), "cascade_gauge", 37,
			day.Add(10*time.Hour), nil, nil,
		)
		refreshAggregates(t)

		dropAllRawChunks(t)

		var rawCount int
		require.NoError(t, testDB.QueryRow(`SELECT count(*) FROM usage_observations`).Scan(&rawCount))
		require.Equal(t, 0, rawCount, "the raw chunk was not actually dropped; the test below proves nothing")

		for _, granularity := range []ct.UsageGranularity{ct.MINUTE, ct.HOUR, ct.DAY} {
			series := w.Usage().Series(seriesQuery("cascade_gauge", granularity, day, day.AddDate(0, 0, 1)))
			require.Lenf(t, series.Items, 1, "%s lost its data when the raw chunk was dropped", granularity)
			assert.InDelta(t, 37.0, series.Items[0].Value, 0)
		}
	})
}

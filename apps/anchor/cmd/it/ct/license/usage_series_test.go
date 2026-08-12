package license_ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageSeries(t *testing.T) {
	t.Run("an empty series reads back as zero items, not an error", func(t *testing.T) {
		w := newLicenseWorld(t)

		series := w.Usage().Series(seriesQuery(
			"flows", ct.MINUTE, time.Now().Add(-time.Hour), time.Now().Add(time.Hour),
		))

		assert.Equal(t, int64(0), series.Total)
		assert.Equal(t, 0, series.Count)
		assert.Empty(t, series.Items)
	})

	t.Run("a reported gauge is readable back through the series", func(t *testing.T) {
		w := newLicenseWorld(t)
		before := time.Now().Add(-time.Minute)

		w.Usage().Report(gauge("flows", 37))
		refreshAggregates(t)

		series := w.Usage().Series(seriesQuery("flows", ct.MINUTE, before, time.Now().Add(time.Minute)))

		require.Len(t, series.Items, 1)
		assert.InDelta(t, 37.0, series.Items[0].Value, 0)
		assert.Nil(t, series.Items[0].From)
		assert.Nil(t, series.Items[0].To)
		assert.Equal(t, int64(1), series.Total)
		assert.Equal(t, 1, series.Count)
	})

	t.Run("a windowed counter's window rides along with its bucket", func(t *testing.T) {
		w := newLicenseWorld(t)
		before := time.Now().Add(-time.Minute)
		from, to := billingPeriod()

		w.Usage().Report(windowed(412, from, to))
		refreshAggregates(t)

		series := w.Usage().Series(seriesQuery("flows", ct.MINUTE, before, time.Now().Add(time.Minute)))

		require.Len(t, series.Items, 1)
		assert.InDelta(t, 412.0, series.Items[0].Value, 0)
		require.NotNil(t, series.Items[0].From)
		require.NotNil(t, series.Items[0].To)
		assert.WithinDuration(t, from, *series.Items[0].From, time.Microsecond)
		assert.WithinDuration(t, to, *series.Items[0].To, time.Microsecond)
	})

	t.Run("filters by key: another field's usage does not leak in", func(t *testing.T) {
		w := newLicenseWorld(t)
		before := time.Now().Add(-time.Minute)
		w.Usage().Report(gauge("flows", 37))

		series := w.Usage().Series(seriesQuery("sso", ct.MINUTE, before, time.Now().Add(time.Minute)))

		assert.Empty(t, series.Items)
	})

	t.Run("filters by time range: a point outside the window is excluded", func(t *testing.T) {
		w := newLicenseWorld(t)
		w.Usage().Report(gauge("flows", 37))
		longAgo := time.Now().Add(-48 * time.Hour)

		series := w.Usage().Series(seriesQuery(
			"flows", ct.MINUTE, longAgo.Add(-time.Hour), longAgo.Add(time.Hour),
		))

		assert.Empty(t, series.Items)
	})

	t.Run("repeated reports within one bucket still read back as one point", func(t *testing.T) {
		w := newLicenseWorld(t)
		before := time.Now().Add(-time.Minute)

		// observed_at is stamped by Anchor at write time, so reports made in quick
		// succession land in the same minute bucket regardless of how many there
		// are. Pagination and ordering across distinct buckets is proved at the
		// aggregate/SQL level in usage_aggregates_test.go, where observed_at can
		// be placed deliberately; the HTTP surface cannot control it.
		w.Usage().Report(gauge("flows", 10))
		w.Usage().Report(gauge("flows", 20))
		w.Usage().Report(gauge("flows", 37))
		refreshAggregates(t)

		series := w.Usage().Series(seriesQuery("flows", ct.MINUTE, before, time.Now().Add(time.Minute)))

		require.Len(t, series.Items, 1)
		assert.InDelta(t, 37.0, series.Items[0].Value, 0)
	})

	t.Run("needs no license, same as reporting", func(t *testing.T) {
		w := newLicenseWorld(t)
		require.Equal(t, http.StatusNotFound, w.License().GetRaw().StatusCode())
		w.Usage().Report(gauge("flows", 37))
		refreshAggregates(t)

		series := w.Usage().Series(seriesQuery(
			"flows", ct.MINUTE, time.Now().Add(-time.Hour), time.Now().Add(time.Hour),
		))

		assert.Len(t, series.Items, 1)
	})

	t.Run("refuses a granularity outside minute, hour or day", func(t *testing.T) {
		w := newLicenseWorld(t)

		// Rejected by the OpenAPI request validator against the contract's own
		// `enum: [MINUTE, HOUR, DAY]`, before the request reaches the handler —
		// so this is a plain-text spec violation, not a domain VALIDATION_ERROR.
		query := seriesQuery("flows", "SECOND", time.Now().Add(-time.Hour), time.Now())
		resp := w.Usage().SeriesRaw(query)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	})

	// "from" has no pointer twin the way the write side's "to" does: the
	// contract marks it required, so the generated client's From is a plain
	// time.Time and cannot encode "omitted" at all — sending nothing sends the
	// zero value, itself a well-formed (if absurd) timestamp. The contract's own
	// `required: true` is what refuses a genuinely missing "from" in production,
	// enforced by the same OpenAPI request validator proved above.

	t.Run("refuses an end at or before the start", func(t *testing.T) {
		w := newLicenseWorld(t)
		from := time.Now()

		resp := w.Usage().SeriesRaw(seriesQuery("flows", ct.MINUTE, from, from))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertValidationRule(t, resp.JSON400.Errors, "gtfield")
	})

	t.Run("refuses an organization the product does not have", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.Usage().For(missingOrganizationID()).SeriesRaw(
			seriesQuery("flows", ct.MINUTE, time.Now().Add(-time.Hour), time.Now()),
		)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

func TestUsageSeriesIsolation(t *testing.T) {
	t.Run("an organization of another product is not addressable", func(t *testing.T) {
		w := newLicenseWorld(t)
		other := newLicenseWorld(t)
		other.Usage().Report(gauge("flows", 37))

		resp := w.Usage().For(other.OrganizationID()).SeriesRaw(
			seriesQuery("flows", ct.MINUTE, time.Now().Add(-time.Hour), time.Now().Add(time.Hour)),
		)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("usage reported for one organization does not appear under another", func(t *testing.T) {
		w := newLicenseWorld(t)
		second := w.NewOrganization()
		w.Usage().Report(gauge("flows", 37))

		series := w.Usage().For(second).Series(seriesQuery(
			"flows", ct.MINUTE, time.Now().Add(-time.Hour), time.Now().Add(time.Hour),
		))

		assert.Empty(t, series.Items)
	})
}

func TestUsageSeriesScopes(t *testing.T) {
	t.Run("a usage-write-only scope cannot read the series", func(t *testing.T) {
		w := newLicenseWorld(t)
		writeOnly, _ := w.product.CreateAPIKeyClientWithScopes([]string{"license_usage:create"})

		resp := w.Usage().As(writeOnly).SeriesRaw(seriesQuery(
			"flows", ct.MINUTE, time.Now().Add(-time.Hour), time.Now(),
		))

		assert.Equal(t, http.StatusForbidden, resp.StatusCode(), string(resp.Body))
	})

	t.Run("a license scope cannot read usage", func(t *testing.T) {
		w := newLicensedWorld(t)
		licenseOnly, _ := w.product.CreateAPIKeyClientWithScopes([]string{"organization_license:read"})

		resp := w.Usage().As(licenseOnly).SeriesRaw(seriesQuery(
			"flows", ct.MINUTE, time.Now().Add(-time.Hour), time.Now(),
		))

		assert.Equal(t, http.StatusForbidden, resp.StatusCode(), string(resp.Body))
	})

	t.Run("the read scope cannot report usage", func(t *testing.T) {
		w := newLicenseWorld(t)
		readOnly, _ := w.product.CreateAPIKeyClientWithScopes([]string{"license_usage:read"})

		resp := w.Usage().As(readOnly).ReportRaw(gauge("flows", 37))

		assert.Equal(t, http.StatusForbidden, resp.StatusCode(), string(resp.Body))
	})
}

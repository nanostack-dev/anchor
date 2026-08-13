package license_ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportUsage(t *testing.T) {
	t.Run("stores a gauge against a declared limit", func(t *testing.T) {
		w := newLicenseWorld(t)

		observation := w.Usage().Report(gauge("flows", 37))

		assert.Equal(t, "flows", observation.Key)
		assert.InDelta(t, 37.0, observation.Value, 0)
		assert.Equal(t, w.OrganizationID(), observation.OrganizationId)
		assert.NotEmpty(t, observation.Id)
		assert.WithinDuration(t, time.Now(), observation.ObservedAt, time.Minute)
		assert.Nil(t, observation.From)
		assert.Nil(t, observation.To)
	})

	t.Run("stores a windowed counter over the period it names", func(t *testing.T) {
		w := newWindowedCounterWorld(t)
		from, to := billingPeriod()

		observation := w.Usage().Report(windowed(worldWindowedCounterKey, 412, from, to))

		assert.InDelta(t, 412.0, observation.Value, 0)
		require.NotNil(t, observation.From)
		require.NotNil(t, observation.To)
		// Postgres timestamptz stores microsecond precision; Go's clock carries
		// nanoseconds. A round trip through storage can never satisfy Equal, so
		// the comparison is bounded to the precision storage actually keeps.
		assert.WithinDuration(t, from, *observation.From, time.Microsecond)
		assert.WithinDuration(t, to, *observation.To, time.Microsecond)
	})

	t.Run("a window left open runs to now", func(t *testing.T) {
		w := newWindowedCounterWorld(t)
		from := time.Now().Add(-24 * time.Hour)

		observation := w.Usage().Report(openEnded(worldWindowedCounterKey, 412, from))

		require.NotNil(t, observation.From)
		require.NotNil(t, observation.To)
		// Same storage-precision bound as above: from is a client-supplied
		// time.Now(), and only billingPeriod's zero-nanosecond fixture happens
		// to survive an exact Equal.
		assert.WithinDuration(t, from, *observation.From, time.Microsecond)
		assert.Equal(t, observation.ObservedAt, *observation.To)
	})

	t.Run("zero is a real observation", func(t *testing.T) {
		w := newLicenseWorld(t)

		observation := w.Usage().Report(gauge("flows", 0))

		assert.InDelta(t, 0.0, observation.Value, 0)
	})

	// The two cases this whole subsystem turns on. Refusing either would make
	// the "exceeded" status unreachable.
	t.Run("accepts a value past the limit the organization holds", func(t *testing.T) {
		w := newLicensedWorld(t)
		require.InDelta(t, 500.0, w.License().Get().Values["flows"], 0)

		observation := w.Usage().Report(gauge("flows", 9000))

		assert.InDelta(t, 9000.0, observation.Value, 0)
	})

	t.Run("accepts a value past the rule the license field declares", func(t *testing.T) {
		w := newLicenseWorld(t)

		observation := w.Usage().Report(gauge("flows", 250_000))

		assert.InDelta(t, 250_000.0, observation.Value, 0)
	})

	t.Run("reporting the same value twice drifts nothing", func(t *testing.T) {
		w := newLicenseWorld(t)

		first := w.Usage().Report(gauge("flows", 37))
		second := w.Usage().Report(gauge("flows", 37))

		assert.NotEqual(t, first.Id, second.Id)
		assert.InDelta(t, first.Value, second.Value, 0)
	})

	t.Run("refuses a key the license schema does not declare", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.Usage().ReportRaw(gauge("flowz", 37))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_UNKNOWN", "flowz", "")
	})

	t.Run("refuses a license field that is not a limit", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.Usage().ReportRaw(gauge("sso", 1))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_NOT_A_LIMIT", "sso", "")
		require.NotNil(t, resp.JSON400.Errors[0].Metadata)
		assert.Equal(t, "BOOLEAN", (*resp.JSON400.Errors[0].Metadata)["type"])
	})

	t.Run("refuses a negative value", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.Usage().ReportRaw(gauge("flows", -1))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertValidationRule(t, resp.JSON400.Errors, "gte")
	})

	t.Run("refuses a value that is not a number", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.Usage().ReportBodyRaw(`{"key":"flows","value":"many"}`)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	})

	// A report that carries no value must not be read as a report of zero.
	// Zero is a real observation, so the omission cannot be caught by a bound
	// on the number, and the request validator does not check the body.
	t.Run("refuses a report that carries no value", func(t *testing.T) {
		w := newLicenseWorld(t)

		omitted := w.Usage().ReportBodyRaw(`{"key":"flows"}`)
		require.Equal(t, http.StatusBadRequest, omitted.StatusCode(), string(omitted.Body))
		assertValidationRule(t, omitted.JSON400.Errors, "required")

		null := w.Usage().ReportBodyRaw(`{"key":"flows","value":null}`)
		require.Equal(t, http.StatusBadRequest, null.StatusCode(), string(null.Body))
		assertValidationRule(t, null.JSON400.Errors, "required")
	})

	// The counterpart to the case above: the guard rejects an absent value, not
	// a zero one, so an organization that genuinely uses nothing still reports.
	t.Run("a report of zero is stored, not mistaken for an omission", func(t *testing.T) {
		w := newLicenseWorld(t)

		observation := w.Usage().Report(gauge("flows", 0))

		assert.InDelta(t, 0.0, observation.Value, 0)
	})

	t.Run("refuses an end with no start", func(t *testing.T) {
		w := newLicenseWorld(t)
		_, to := billingPeriod()

		resp := w.Usage().ReportRaw(ct.UsageReportRequest{Key: "flows", Value: 412, To: &to})

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertValidationRule(t, resp.JSON400.Errors, "required_with")
	})

	t.Run("refuses a window that holds no time", func(t *testing.T) {
		w := newWindowedCounterWorld(t)
		from, to := billingPeriod()

		empty := w.Usage().ReportRaw(windowed(worldWindowedCounterKey, 412, from, from))
		require.Equal(t, http.StatusBadRequest, empty.StatusCode(), string(empty.Body))
		assertValidationRule(t, empty.JSON400.Errors, "gtfield")

		reversed := w.Usage().ReportRaw(windowed(worldWindowedCounterKey, 412, to, from))
		require.Equal(t, http.StatusBadRequest, reversed.StatusCode(), string(reversed.Body))
		assertValidationRule(t, reversed.JSON400.Errors, "gtfield")
	})

	t.Run("refuses a window longer than a year", func(t *testing.T) {
		w := newWindowedCounterWorld(t)
		_, to := billingPeriod()

		resp := w.Usage().ReportRaw(windowed(worldWindowedCounterKey, 412, to.AddDate(-1, 0, -1), to))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertAPIError(t, resp.JSON400.Errors, "USAGE_WINDOW_TOO_LONG")
	})

	t.Run("accepts a window of exactly a year", func(t *testing.T) {
		w := newWindowedCounterWorld(t)
		_, to := billingPeriod()

		observation := w.Usage().Report(windowed(worldWindowedCounterKey, 8_640, to.AddDate(-1, 0, 0), to))

		assert.InDelta(t, 8_640.0, observation.Value, 0)
	})

	t.Run("a window left open cannot outrun the year bound", func(t *testing.T) {
		w := newWindowedCounterWorld(t)

		resp := w.Usage().ReportRaw(
			openEnded(worldWindowedCounterKey, 412, time.Now().AddDate(-2, 0, 0)),
		)

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertAPIError(t, resp.JSON400.Errors, "USAGE_WINDOW_TOO_LONG")
	})

	t.Run("refuses an organization the product does not have", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.Usage().For(missingOrganizationID()).ReportRaw(gauge("flows", 37))

		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("refuses a product that has declared no license schema", func(t *testing.T) {
		tc := newTestCtx(t)
		organizationID := createOrganization(t, tc.product)

		resp, err := tc.product.OwnerAuthenticatedClient().ReportOrganizationUsageWithResponse(
			context.Background(), tc.product.ProductID, organizationID, gauge("flows", 37),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

// TestReportUsageShape covers the enforcement declareFields pins on
// declaration: a report's window presence must agree with the field's
// declared usage_shape. See docs/adr/0012-usage-shape-is-declared-not-inferred.md.
func TestReportUsageShape(t *testing.T) {
	t.Run("refuses a windowed report against a gauge field", func(t *testing.T) {
		w := newLicenseWorld(t)
		from, to := billingPeriod()

		resp := w.Usage().ReportRaw(windowed(worldLimitKey, 412, from, to))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertFieldError(t, resp.JSON400.Errors, "USAGE_SHAPE_MISMATCH", worldLimitKey, "")
		require.NotNil(t, resp.JSON400.Errors[0].Metadata)
		assert.Equal(t, "GAUGE", (*resp.JSON400.Errors[0].Metadata)["usage_shape"])
	})

	t.Run("refuses an open-ended report against a gauge field", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.Usage().ReportRaw(openEnded(worldLimitKey, 412, time.Now().Add(-time.Hour)))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertFieldError(t, resp.JSON400.Errors, "USAGE_SHAPE_MISMATCH", worldLimitKey, "")
	})

	t.Run("refuses a windowless report against a windowed counter field", func(t *testing.T) {
		w := newWindowedCounterWorld(t)

		resp := w.Usage().ReportRaw(gauge(worldWindowedCounterKey, 37))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertFieldError(t, resp.JSON400.Errors, "USAGE_SHAPE_MISMATCH", worldWindowedCounterKey, "")
		require.NotNil(t, resp.JSON400.Errors[0].Metadata)
		assert.Equal(t, "WINDOWED_COUNTER", (*resp.JSON400.Errors[0].Metadata)["usage_shape"])
	})

	// A field declared before this migration carries no usage_shape at all —
	// the column has no backfill. resolveLimit refuses to guess one rather
	// than defaulting to gauge or counter, either of which would be a silent
	// answer to a question only the field's owner can settle.
	t.Run("refuses usage against a limit with no usage_shape declared", func(t *testing.T) {
		w := newWindowedCounterWorld(t)
		// Scoped to this world's own schema: a field name is unique only within
		// its schema, and every newWindowedCounterWorld declares a
		// worldWindowedCounterKey — an update by name alone would blank the
		// shape on every other world's copy too, breaking whichever test runs
		// next.
		_, err := testDB.Exec(
			`UPDATE license_schema_fields SET usage_shape = NULL
			 WHERE license_schema_id = $1 AND name = $2`,
			w.SchemaID(), worldWindowedCounterKey,
		)
		require.NoError(t, err)

		resp := w.Usage().ReportRaw(gauge(worldWindowedCounterKey, 37))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_USAGE_SHAPE_UNDECLARED", worldWindowedCounterKey, "")
	})
}

func TestReportUsageIsolation(t *testing.T) {
	t.Run("an organization of another product is not addressable", func(t *testing.T) {
		w := newLicenseWorld(t)
		other := newLicenseWorld(t)

		resp := w.Usage().For(other.OrganizationID()).ReportRaw(gauge("flows", 37))

		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("usage is reported against the organization in the path", func(t *testing.T) {
		w := newLicenseWorld(t)
		second := w.NewOrganization()

		first := w.Usage().Report(gauge("flows", 37))
		other := w.Usage().For(second).Report(gauge("flows", 412))

		assert.Equal(t, w.OrganizationID(), first.OrganizationId)
		assert.Equal(t, second, other.OrganizationId)
	})
}

func TestReportUsageNeedsNoLicense(t *testing.T) {
	t.Run("an organization on no tier can report", func(t *testing.T) {
		w := newLicenseWorld(t)
		require.Equal(t, http.StatusNotFound, w.License().GetRaw().StatusCode())

		observation := w.Usage().Report(gauge("flows", 37))

		assert.InDelta(t, 37.0, observation.Value, 0)
	})

	t.Run("an organization whose tier was withdrawn keeps reporting", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.Template().Archive()

		observation := w.Usage().Report(gauge("flows", 37))

		assert.InDelta(t, 37.0, observation.Value, 0)
	})
}

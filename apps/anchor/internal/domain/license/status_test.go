package license_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/license"
	"anchor/internal/license/rules"
)

// statusNow is the fixed clock every case below is measured against, so a
// boundary between "reported recently" and "stale" is an exact value rather
// than an approximate one.
var statusNow = time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)

func limitField(expectedReportingInterval *time.Duration) license.Field {
	return license.Field{Name: "flows", Type: rules.Limit, ExpectedReportingInterval: expectedReportingInterval}
}

func observationAt(value float64, observedAt time.Time) license.UsageObservation {
	return license.UsageObservation{Key: "flows", Value: value, ObservedAt: observedAt}
}

// TestDeriveUsage walks every status transition, including the two shapes of
// staleness: never having reported, and having reported too long ago. Each
// case is pure — no database, no clock read — so the boundary between
// "reported recently" and "stale" is pinned exactly.
func TestDeriveUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fields      []license.Field
		values      license.TemplateValues
		latestByKey map[string]license.UsageObservation
		want        map[string]license.FieldUsage
	}{
		{
			name:   "within_limit: latest usage under the limit",
			fields: []license.Field{limitField(nil)},
			values: license.TemplateValues{"flows": 500.0},
			latestByKey: map[string]license.UsageObservation{
				"flows": observationAt(400, statusNow.Add(-time.Minute)),
			},
			want: map[string]license.FieldUsage{
				"flows": {
					Field: "flows", Limit: 500, Usage: new(400.0),
					Status: license.UsageWithinLimit, LastReportedAt: new(statusNow.Add(-time.Minute)),
				},
			},
		},
		{
			name:   "at_limit: latest usage equal to the limit",
			fields: []license.Field{limitField(nil)},
			values: license.TemplateValues{"flows": 500.0},
			latestByKey: map[string]license.UsageObservation{
				"flows": observationAt(500, statusNow.Add(-time.Minute)),
			},
			want: map[string]license.FieldUsage{
				"flows": {
					Field: "flows", Limit: 500, Usage: new(500.0),
					Status: license.UsageAtLimit, LastReportedAt: new(statusNow.Add(-time.Minute)),
				},
			},
		},
		{
			name:   "exceeded: latest usage past the limit",
			fields: []license.Field{limitField(nil)},
			values: license.TemplateValues{"flows": 500.0},
			latestByKey: map[string]license.UsageObservation{
				"flows": observationAt(9000, statusNow.Add(-time.Minute)),
			},
			want: map[string]license.FieldUsage{
				"flows": {
					Field: "flows", Limit: 500, Usage: new(9000.0),
					Status: license.UsageExceeded, LastReportedAt: new(statusNow.Add(-time.Minute)),
				},
			},
		},
		{
			name:        "stale: nothing has ever been reported",
			fields:      []license.Field{limitField(nil)},
			values:      license.TemplateValues{"flows": 500.0},
			latestByKey: map[string]license.UsageObservation{},
			want: map[string]license.FieldUsage{
				"flows": {Field: "flows", Limit: 500, Usage: nil, Status: license.UsageStale, LastReportedAt: nil},
			},
		},
		{
			name:   "stale: reported, but no expected reporting interval declared never ages out",
			fields: []license.Field{limitField(nil)},
			values: license.TemplateValues{"flows": 500.0},
			latestByKey: map[string]license.UsageObservation{
				"flows": observationAt(400, statusNow.Add(-1000*time.Hour)),
			},
			want: map[string]license.FieldUsage{
				"flows": {
					Field: "flows", Limit: 500, Usage: new(400.0),
					Status: license.UsageWithinLimit, LastReportedAt: new(statusNow.Add(-1000 * time.Hour)),
				},
			},
		},
		{
			name:   "stale: latest observation older than the declared expected reporting interval",
			fields: []license.Field{limitField(new(time.Hour))},
			values: license.TemplateValues{"flows": 500.0},
			latestByKey: map[string]license.UsageObservation{
				"flows": observationAt(400, statusNow.Add(-2*time.Hour)),
			},
			want: map[string]license.FieldUsage{
				"flows": {
					Field: "flows", Limit: 500, Usage: new(400.0),
					Status: license.UsageStale, LastReportedAt: new(statusNow.Add(-2 * time.Hour)),
				},
			},
		},
		{
			name:   "not stale: latest observation exactly at the boundary of the expected reporting interval",
			fields: []license.Field{limitField(new(time.Hour))},
			values: license.TemplateValues{"flows": 500.0},
			latestByKey: map[string]license.UsageObservation{
				"flows": observationAt(400, statusNow.Add(-time.Hour)),
			},
			want: map[string]license.FieldUsage{
				"flows": {
					Field: "flows", Limit: 500, Usage: new(400.0),
					Status: license.UsageWithinLimit, LastReportedAt: new(statusNow.Add(-time.Hour)),
				},
			},
		},
		{
			name:   "stale: one instant past the expected reporting interval",
			fields: []license.Field{limitField(new(time.Hour))},
			values: license.TemplateValues{"flows": 500.0},
			latestByKey: map[string]license.UsageObservation{
				"flows": observationAt(400, statusNow.Add(-time.Hour-time.Nanosecond)),
			},
			want: map[string]license.FieldUsage{
				"flows": {
					Field: "flows", Limit: 500, Usage: new(400.0),
					Status: license.UsageStale, LastReportedAt: new(statusNow.Add(-time.Hour - time.Nanosecond)),
				},
			},
		},
		{
			name: "a field that is not a limit never appears in the usage map",
			fields: []license.Field{
				limitField(nil),
				{Name: "sso", Type: rules.Boolean},
			},
			values: license.TemplateValues{"flows": 500.0, "sso": true},
			latestByKey: map[string]license.UsageObservation{
				"flows": observationAt(400, statusNow.Add(-time.Minute)),
			},
			want: map[string]license.FieldUsage{
				"flows": {
					Field: "flows", Limit: 500, Usage: new(400.0),
					Status: license.UsageWithinLimit, LastReportedAt: new(statusNow.Add(-time.Minute)),
				},
			},
		},
		{
			name:        "a limit field with no resolvable value is skipped defensively",
			fields:      []license.Field{limitField(nil)},
			values:      license.TemplateValues{},
			latestByKey: map[string]license.UsageObservation{},
			want:        map[string]license.FieldUsage{},
		},
		{
			name: "an adjusted limit turns a previously-compliant organization into exceeded",
			fields: []license.Field{
				limitField(nil),
			},
			// The limit dropped from 500 to 300 without a new report — status
			// is derived against the current limit, not the one that was in
			// effect when the observation was recorded.
			values: license.TemplateValues{"flows": 300.0},
			latestByKey: map[string]license.UsageObservation{
				"flows": observationAt(400, statusNow.Add(-time.Minute)),
			},
			want: map[string]license.FieldUsage{
				"flows": {
					Field: "flows", Limit: 300, Usage: new(400.0),
					Status: license.UsageExceeded, LastReportedAt: new(statusNow.Add(-time.Minute)),
				},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := license.DeriveUsage(testCase.fields, testCase.values, testCase.latestByKey, statusNow)

			assert.Equal(t, testCase.want, got)
		})
	}
}

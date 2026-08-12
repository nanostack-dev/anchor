package license_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/license"
	"anchor/internal/license/rules"
)

var statusNow = time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)

func limitField() license.Field {
	return license.Field{Name: "flows", Type: rules.Limit}
}

func observationAt(value float64, observedAt time.Time) license.UsageObservation {
	return license.UsageObservation{Key: "flows", Value: value, ObservedAt: observedAt}
}

// TestDeriveUsage walks every status transition, including staleness from
// never having reported. Each case is pure — no database, no clock read.
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
			fields: []license.Field{limitField()},
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
			fields: []license.Field{limitField()},
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
			fields: []license.Field{limitField()},
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
			fields:      []license.Field{limitField()},
			values:      license.TemplateValues{"flows": 500.0},
			latestByKey: map[string]license.UsageObservation{},
			want: map[string]license.FieldUsage{
				"flows": {Field: "flows", Limit: 500, Usage: nil, Status: license.UsageStale, LastReportedAt: nil},
			},
		},
		{
			name:   "an old observation still reads within_limit — there is no expected reporting interval",
			fields: []license.Field{limitField()},
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
			name: "a field that is not a limit never appears in the usage map",
			fields: []license.Field{
				limitField(),
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
			fields:      []license.Field{limitField()},
			values:      license.TemplateValues{},
			latestByKey: map[string]license.UsageObservation{},
			want:        map[string]license.FieldUsage{},
		},
		{
			name:   "an adjusted limit turns a previously-compliant organization into exceeded",
			fields: []license.Field{limitField()},
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

			got := license.DeriveUsage(testCase.fields, testCase.values, testCase.latestByKey)

			assert.Equal(t, testCase.want, got)
		})
	}
}

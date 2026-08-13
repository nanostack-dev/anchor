package license_test

import (
	"math"
	"testing"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/license"
)

// reportedAt is the fixed clock every case is normalised against, so a
// defaulted window end is an exact value rather than an approximate one.
var reportedAt = time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)

func at(offset time.Duration) *time.Time {
	moment := reportedAt.Add(offset)
	return &moment
}

// screen runs the three steps ReportUsage runs before it touches a repository,
// so the table below covers the pipeline rather than one half of it.
func screen(in license.ReportUsageInput) (license.ReportUsageInput, error) {
	report := in.WithDefaults(reportedAt)
	if err := validate.ValidateStruct(report); err != nil {
		return report, err
	}
	return report, report.Check()
}

func TestReportUsageInputScreening(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value float64
		from  *time.Time
		to    *time.Time
		// One of these is set on a rejected case: rule for what the validate
		// tags catch, err for the two rules a tag cannot express.
		rule         string
		err          error
		expectedFrom *time.Time
		expectedTo   *time.Time
	}{
		{
			name:  "a gauge carries no window",
			value: 37,
		},
		{
			name:         "a windowed counter carries both ends",
			value:        412,
			from:         at(-8 * time.Hour),
			to:           at(0),
			expectedFrom: at(-8 * time.Hour),
			expectedTo:   at(0),
		},
		{
			name:  "zero is a real observation",
			value: 0,
		},
		{
			name:         "a start on its own runs to now",
			value:        412,
			from:         at(-8 * time.Hour),
			expectedFrom: at(-8 * time.Hour),
			expectedTo:   at(0),
		},
		{
			name:  "an end on its own has no start to run from",
			value: 412,
			to:    at(0),
			rule:  "required_with",
		},
		{
			name:  "a negative value is not usage",
			value: -1,
			rule:  "gte",
		},
		{
			name:  "a value that is not a number is refused",
			value: math.NaN(),
			rule:  "gte",
		},
		{
			name:  "an infinite value is refused",
			value: math.Inf(1),
			err:   license.ErrUsageValueNotFinite,
		},
		{
			name:  "a window that starts where it ends holds no time",
			value: 412,
			from:  at(0),
			to:    at(0),
			rule:  "gtfield",
		},
		{
			name:  "a window that ends before it starts holds no time",
			value: 412,
			from:  at(time.Hour),
			to:    at(0),
			rule:  "gtfield",
		},
		{
			name:         "a window of exactly one year is allowed",
			value:        412,
			from:         at(-oneYear),
			to:           at(0),
			expectedFrom: at(-oneYear),
			expectedTo:   at(0),
		},
		{
			name:  "a window longer than one year is refused",
			value: 412,
			from:  at(-oneYear - 24*time.Hour),
			to:    at(0),
			err:   license.ErrUsageWindowTooLong,
		},
		{
			name:  "a value far past any limit is accepted",
			value: 9_000_000,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			report, err := screen(license.ReportUsageInput{
				TenantID:       "tenant_x",
				ProductID:      "prd_x",
				OrganizationID: "org_x",
				Key:            "flows",
				Value:          testCase.value,
				From:           testCase.from,
				To:             testCase.to,
			})

			switch {
			case testCase.rule != "":
				assertValidationRule(t, err, testCase.rule)
			case testCase.err != nil:
				require.ErrorIs(t, err, testCase.err)
			default:
				require.NoError(t, err)
				assert.Equal(t, testCase.expectedFrom, report.From)
				assert.Equal(t, testCase.expectedTo, report.To)
			}
		})
	}
}

// assertValidationRule names the validate tag that refused the report. The tag
// travels as metadata rather than a distinct code, so this is what tells
// "negative value" apart from "window ends before it starts".
func assertValidationRule(t *testing.T, err error, rule string) {
	t.Helper()
	var validationErr *fault.Error
	require.ErrorAs(t, err, &validationErr)
	require.NotEmpty(t, validationErr.Details)
	assert.Equal(t, "VALIDATION_ERROR", validationErr.Details[0].Code)
	assert.Equal(t, rule, validationErr.Details[0].Metadata["rule"])
}

// A calendar year, not a fixed count of hours, because that is what the check
// itself measures.
var oneYear = reportedAt.Sub(reportedAt.AddDate(-1, 0, 0))

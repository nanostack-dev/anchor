package license_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/license"
)

// reportedAt is the fixed clock every case below is normalised against, so a
// defaulted window end is an exact value rather than an approximate one.
var reportedAt = time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)

func at(offset time.Duration) *time.Time {
	moment := reportedAt.Add(offset)
	return &moment
}

// TestReportUsageInputNormalize pins every way a usage report is malformed on
// its own terms, and the one value Anchor fills in — before a schema is loaded
// and without a database round trip.
//
// The last case is the one this whole subsystem turns on: a value far past any
// limit the field could declare is accepted here. Rules bound what a limit may
// be set to and are never applied to an observation. Applying them would make
// the "exceeded" status unreachable, and Anchor would keep serving a stale
// value reading "within_limit".
func TestReportUsageInputNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		value        float64
		from         *time.Time
		to           *time.Time
		expected     error
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
			name:     "an end on its own has no start to run from",
			value:    412,
			to:       at(0),
			expected: license.ErrUsageWindowIncomplete,
		},
		{
			name:     "a negative value is not usage",
			value:    -1,
			expected: license.ErrUsageValueNegative,
		},
		{
			name:     "a value that is not a number is refused",
			value:    math.NaN(),
			expected: license.ErrUsageValueNotFinite,
		},
		{
			name:     "an infinite value is refused",
			value:    math.Inf(1),
			expected: license.ErrUsageValueNotFinite,
		},
		{
			name:     "a window that starts where it ends holds no time",
			value:    412,
			from:     at(0),
			to:       at(0),
			expected: license.ErrUsageWindowEmpty,
		},
		{
			name:     "a window that ends before it starts holds no time",
			value:    412,
			from:     at(time.Hour),
			to:       at(0),
			expected: license.ErrUsageWindowEmpty,
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
			name:     "a window longer than one year is refused",
			value:    412,
			from:     at(-oneYear - 24*time.Hour),
			to:       at(0),
			expected: license.ErrUsageWindowTooLong,
		},
		{
			name:  "a value far past any limit is accepted",
			value: 9_000_000,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			normalized, err := license.ReportUsageInput{
				Value: testCase.value,
				From:  testCase.from,
				To:    testCase.to,
			}.Normalize(reportedAt)

			if testCase.expected != nil {
				assert.ErrorIs(t, err, testCase.expected)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.expectedFrom, normalized.From)
			assert.Equal(t, testCase.expectedTo, normalized.To)
		})
	}
}

// oneYear is the span the cases above straddle. It is the calendar year ending
// at reportedAt, not a fixed count of hours, because that is what the check
// itself measures.
var oneYear = reportedAt.Sub(reportedAt.AddDate(-1, 0, 0))

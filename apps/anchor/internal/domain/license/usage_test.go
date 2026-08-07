package license_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/license"
)

func at(hour int) *time.Time {
	moment := time.Date(2026, time.August, 14, hour, 0, 0, 0, time.UTC)
	return &moment
}

// TestReportUsageInputCheck pins every way a usage report can be malformed on
// its own terms — before a schema is loaded and without a database round trip.
//
// The last case is the one this whole subsystem turns on: a value far past any
// limit the field could declare is accepted here. Rules bound what a limit may
// be set to and are never applied to an observation. Applying them would make
// the "exceeded" status unreachable, and Anchor would keep serving a stale
// value reading "within_limit".
func TestReportUsageInputCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    float64
		start    *time.Time
		end      *time.Time
		expected error
	}{
		{
			name:  "a gauge carries no window",
			value: 37,
		},
		{
			name:  "a windowed counter carries both ends",
			value: 412,
			start: at(9),
			end:   at(17),
		},
		{
			name:  "zero is a real observation",
			value: 0,
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
			name:     "a start without an end is half a window",
			value:    412,
			start:    at(9),
			expected: license.ErrUsageWindowIncomplete,
		},
		{
			name:     "an end without a start is half a window",
			value:    412,
			end:      at(17),
			expected: license.ErrUsageWindowIncomplete,
		},
		{
			name:     "a window that starts where it ends holds no time",
			value:    412,
			start:    at(9),
			end:      at(9),
			expected: license.ErrUsageWindowEmpty,
		},
		{
			name:     "a window that ends before it starts holds no time",
			value:    412,
			start:    at(17),
			end:      at(9),
			expected: license.ErrUsageWindowEmpty,
		},
		{
			name:  "a value far past any limit is accepted",
			value: 9_000_000,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := license.ReportUsageInput{
				Value:       testCase.value,
				WindowStart: testCase.start,
				WindowEnd:   testCase.end,
			}.Check()

			if testCase.expected == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, testCase.expected)
		})
	}
}

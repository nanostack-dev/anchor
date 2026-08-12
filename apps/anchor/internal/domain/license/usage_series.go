package license

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

// UsageGranularity selects a level of the usage-observation continuous
// aggregate cascade: minute, hour or day, each built on the one before it.
// See docs/adr/0005-timescaledb-for-usage-history.md.
type UsageGranularity string

const (
	UsageGranularityMinute UsageGranularity = "MINUTE"
	UsageGranularityHour   UsageGranularity = "HOUR"
	UsageGranularityDay    UsageGranularity = "DAY"
)

// UsageSeriesPoint is one bucket of a read series: the last observation's
// value in that bucket, at whatever granularity was requested. From and To
// are both set or both nil, same as [UsageObservation] — carried through the
// aggregate unchanged from the last report the bucket was built from.
type UsageSeriesPoint struct {
	Bucket     time.Time
	Value      float64
	WindowFrom *time.Time
	WindowTo   *time.Time
}

// GetUsageSeriesInput is a request to read one Organization's usage history
// for one license field, filtered by time range and paginated.
//
// It carries no evaluator and no limit: interpreting a series against a
// limit stays in the service layer, and this input does not even reach that
// layer — it is answered by shaping and returning stored aggregate data.
type GetUsageSeriesInput struct {
	TenantID       string            `validate:"required,notblank"`
	ProductID      string            `validate:"required,notblank"`
	OrganizationID string            `validate:"required,notblank"`
	Key            string            `validate:"required,notblank,max=120"`
	Granularity    UsageGranularity  `validate:"required,oneof=MINUTE HOUR DAY"`
	From           *time.Time        `validate:"required"`
	To             *time.Time        `validate:"omitempty,gtfield=From"`
	Pagination     search.Pagination `validate:"required"`
}

// WithDefaults ends an open range at now, mirroring [ReportUsageInput]'s own
// "omit To to mean now". It takes the clock rather than reading it, so a
// defaulted To is an exact value in a test.
func (in GetUsageSeriesInput) WithDefaults(now time.Time) GetUsageSeriesInput {
	if in.To == nil {
		in.To = &now
	}
	return in
}

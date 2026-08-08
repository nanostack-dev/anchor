package license

import (
	"errors"
	"math"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// The two rules the validate tags on [ReportUsageInput] cannot express.
var (
	ErrUsageValueNotFinite = errors.New("a reported usage value must be a finite number")
	ErrUsageWindowTooLong  = errors.New("a usage window cannot be longer than one year")
)

// UsageObservation is one stored usage report. From and To are both set or both
// nil: nil is a gauge, set is a counter over the half-open period [From, To).
type UsageObservation struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	OrganizationID   string
	Key              string
	Value            float64
	From             *time.Time
	To               *time.Time
	ObservedAt       time.Time
}

func (o *UsageObservation) GenerateID() {
	o.ID = ids.MustNew("uobs")
}

// ReportUsageInput is one absolute snapshot a consumer sends.
//
// The tags are checked after [ReportUsageInput.WithDefaults] has run, so
// gtfield sees a To that was filled in rather than one the caller omitted.
//
// gte on Value is the only bound there is, and it exists because usage cannot
// be negative — never because a limit says so. A report past the organization's
// limit must be stored, or the "exceeded" status becomes unreachable.
type ReportUsageInput struct {
	TenantID       string     `validate:"required,notblank"`
	ProductID      string     `validate:"required,notblank"`
	OrganizationID string     `validate:"required,notblank"`
	Key            string     `validate:"required,notblank,max=120"`
	Value          float64    `validate:"gte=0"`
	From           *time.Time `validate:"required_with=To"`
	To             *time.Time `validate:"omitempty,gtfield=From"`
}

// WithDefaults ends an open window at now. It takes the clock rather than
// reading it, so the filled value is exact in a test.
func (in ReportUsageInput) WithDefaults(now time.Time) ReportUsageInput {
	if in.From != nil && in.To == nil {
		in.To = &now
	}
	return in
}

// Check covers what the tags cannot. gte admits +Inf, and Postgres stores it,
// which would poison every aggregate built over the series. No tag compares a
// duration between two fields, so the one-year bound is measured here — as a
// calendar year, so a window crossing a leap day is measured the way a customer
// would describe it.
func (in ReportUsageInput) Check() error {
	if math.IsInf(in.Value, 0) {
		return ErrUsageValueNotFinite
	}
	if in.From != nil && in.To != nil && in.To.After(in.From.AddDate(1, 0, 0)) {
		return ErrUsageWindowTooLong
	}
	return nil
}

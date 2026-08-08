package license

import (
	"errors"
	"math"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

var (
	ErrUsageValueNegative    = errors.New("a reported usage value cannot be negative")
	ErrUsageValueNotFinite   = errors.New("a reported usage value must be a finite number")
	ErrUsageWindowIncomplete = errors.New("a usage window that ends must say where it starts")
	ErrUsageWindowEmpty      = errors.New("a usage window must start before it ends")
	ErrUsageWindowTooLong    = errors.New("a usage window cannot be longer than one year")
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
type ReportUsageInput struct {
	TenantID       string `validate:"required,notblank"`
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	Key            string `validate:"required,notblank,max=120"`
	Value          float64
	From           *time.Time
	To             *time.Time
}

// MaxUsageWindow is a calendar year rather than a fixed count of hours, so a
// window crossing a leap day is measured the way a customer would describe it.
func MaxUsageWindow(from time.Time) time.Time {
	return from.AddDate(1, 0, 0)
}

// Normalize fills in an absent To and checks everything the report can be
// judged on by itself. It takes the clock rather than reading it, so the checks
// stay pure and a defaulted end is an exact value in a test.
//
// The value is refused only for not being a usable number. How large it is
// never matters: a report past the organization's limit must be stored, or the
// "exceeded" status becomes unreachable. Nothing on this path reads FieldRules.
func (in ReportUsageInput) Normalize(now time.Time) (ReportUsageInput, error) {
	if in.From != nil && in.To == nil {
		in.To = &now
	}

	if math.IsNaN(in.Value) || math.IsInf(in.Value, 0) {
		return in, ErrUsageValueNotFinite
	}
	if in.Value < 0 {
		return in, ErrUsageValueNegative
	}

	if in.From == nil {
		if in.To != nil {
			return in, ErrUsageWindowIncomplete
		}
		return in, nil
	}

	if !in.From.Before(*in.To) {
		return in, ErrUsageWindowEmpty
	}
	if in.To.After(MaxUsageWindow(*in.From)) {
		return in, ErrUsageWindowTooLong
	}

	return in, nil
}

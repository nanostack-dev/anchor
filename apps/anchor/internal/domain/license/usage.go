package license

// What an Organization has actually consumed, against the limits its license
// grants.
//
// A consumer reports an absolute snapshot — "this Organization now has 37
// flows" — and Anchor appends it as an immutable observation. Anchor never
// sums. A retried report is harmless, because the same value landing twice
// changes nothing, and a report that never arrived self-corrects on the next
// one. See docs/adr/0003-usage-reported-as-snapshots.md.
//
// # Rules are never applied here
//
// A license field's rules bound what a limit may be *set* to. They are never
// applied to a value that was *observed*. A report of 512 against a limit of
// 500 is accepted and stored, or the "exceeded" status becomes unreachable and
// Anchor keeps serving a stale value reading "within_limit". Nothing in this
// file reads [FieldRules], and nothing on the usage path may.

import (
	"errors"
	"math"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// The ways a usage report is malformed on its own terms, before a license
// schema is loaded and without a database round trip. They are sentinels rather
// than API errors because this package stays pure: the service maps them onto
// the contract, as it does for a rules violation.
var (
	ErrUsageValueNegative    = errors.New("a reported usage value cannot be negative")
	ErrUsageValueNotFinite   = errors.New("a reported usage value must be a finite number")
	ErrUsageWindowIncomplete = errors.New("a usage window that ends must say where it starts")
	ErrUsageWindowEmpty      = errors.New("a usage window must start before it ends")
	ErrUsageWindowTooLong    = errors.New("a usage window cannot be longer than one year")
)

// UsageObservation is one stored usage report. It is written once and never
// edited: a correction is a new observation, not an overwrite.
type UsageObservation struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	OrganizationID   string
	// Key names the license field this value was reported against. The field is
	// always of type [rules.Limit] — a boolean feature toggle carries no usage.
	Key   string
	Value float64
	// From and To are both set or both nil, which is what distinguishes the two
	// kinds of usage. Nil is a gauge: "37 flows exist right now", a number that
	// rises and falls. Set is a windowed counter over the half-open period
	// [From, To): "412 runs between August 14 and September 14", which resets
	// by starting a new window.
	//
	// Two timestamps rather than a formatted period, because real billing
	// periods follow the subscription anniversary rather than the calendar.
	From *time.Time
	To   *time.Time
	// ObservedAt is when Anchor accepted the report, and the column the
	// hypertable partitions on. Anchor sets it, so a consumer cannot write into
	// the future or rewrite its own history by backdating a report.
	ObservedAt time.Time
}

// GenerateID sets the observation's ID to a new prefixed KSUID. KSUIDs sort by
// creation time, so the identifier doubles as the cursor the usage series is
// paged by.
func (o *UsageObservation) GenerateID() {
	o.ID = ids.MustNew("uobs")
}

// ReportUsageInput is one absolute snapshot a consumer sends. Repeating it is
// safe and expected: reporting cadence is the consumer's decision, not a
// contract shared with Anchor.
type ReportUsageInput struct {
	TenantID       string `validate:"required,notblank"`
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	// Key must name a license field the Product's schema declares, and that
	// field must be a limit. Both are the schema's questions, so the service
	// asks them; neither is answerable here.
	Key   string `validate:"required,notblank,max=120"`
	Value float64
	From  *time.Time
	To    *time.Time
}

// MaxUsageWindow is the longest period one report may cover.
//
// A window longer than a year is a mistyped year far more often than a real
// billing period — no subscription term Anchor serves runs longer than an
// annual one. Raw observations are aged out well before that, so a window this
// side of the bound is one the series can still be read over.
//
// It is a calendar year rather than a fixed count of hours, so a window that
// crosses a leap day is measured the way a customer would describe it.
func MaxUsageWindow(from time.Time) time.Time {
	return from.AddDate(1, 0, 0)
}

// Normalize fills in what Anchor decides and checks everything the report can
// be judged on by itself. It takes the clock rather than reading it, so the
// checks stay pure and a defaulted end is an exact value in a test.
//
// One value is filled in: a report that says where it starts and not where it
// ends runs to now. "412 runs since August 14" is a complete statement, and
// making the caller send a timestamp meaning "now" would only invite clock
// skew. An end with no start is still refused — there is nothing to run from.
//
// A value is refused only for not being a usable number: negative, or not
// finite. How large it is never matters. See the file comment.
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

	// A half-open window that does not start before it ends covers no time at
	// all, so nothing could have been counted in it.
	if !in.From.Before(*in.To) {
		return in, ErrUsageWindowEmpty
	}
	if in.To.After(MaxUsageWindow(*in.From)) {
		return in, ErrUsageWindowTooLong
	}

	return in, nil
}

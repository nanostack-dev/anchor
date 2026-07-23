package webhook

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// EndpointStatus is the lifecycle status of a subscription.
//
// DISABLED is an administrator's decision; AUTO_DISABLED is Anchor's. They are
// kept distinct so an operator can tell "the customer turned this off" from
// "we turned this off because it kept failing", and so re-enabling records the
// right audit intent.
type EndpointStatus string

const (
	EndpointStatusEnabled      EndpointStatus = "ENABLED"
	EndpointStatusDisabled     EndpointStatus = "DISABLED"
	EndpointStatusAutoDisabled EndpointStatus = "AUTO_DISABLED"
)

func (s EndpointStatus) IsValid() bool {
	switch s {
	case EndpointStatusEnabled, EndpointStatusDisabled, EndpointStatusAutoDisabled:
		return true
	default:
		return false
	}
}

const (
	// AutoDisableFailureThreshold is the number of consecutive failed
	// deliveries required before an endpoint may be auto-disabled.
	AutoDisableFailureThreshold int32 = 20

	// AutoDisableMinFailureAge is how long the failure streak must have been
	// running before an endpoint may be auto-disabled.
	AutoDisableMinFailureAge = 24 * time.Hour

	// AutoDisableReason is the stored reason for an automatic disable.
	AutoDisableReason = "Auto-disabled after sustained delivery failures"

	// GoneDisableReason is the stored reason when a receiver answered 410 Gone.
	GoneDisableReason = "Auto-disabled because the receiver answered 410 Gone"
)

// Endpoint is a product-scoped outbound webhook subscription.
type Endpoint struct {
	ID                      string
	ProductID               string
	URL                     string
	Description             string
	EventTypes              []string
	Status                  EndpointStatus
	DisabledReason          string
	ConsecutiveFailureCount int32
	FirstFailureAt          *time.Time
	LastFailureAt           *time.Time
	LastSuccessAt           *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// GenerateID sets the endpoint's ID to a new prefixed KSUID.
func (e *Endpoint) GenerateID() {
	e.ID = ids.MustNew("whe")
}

// IsEnabled reports whether the endpoint currently accrues deliveries.
func (e *Endpoint) IsEnabled() bool {
	return e.Status == EndpointStatusEnabled
}

// Subscribes reports whether this endpoint should receive eventType.
func (e *Endpoint) Subscribes(eventType string) bool {
	return MatchesSubscription(e.EventTypes, eventType)
}

// FailureCounters is the failure-streak state an endpoint carries. It is split
// out from Endpoint so the transition rules below stay pure functions that a
// unit test can drive without constructing a whole aggregate.
type FailureCounters struct {
	ConsecutiveFailureCount int32
	FirstFailureAt          *time.Time
	LastFailureAt           *time.Time
	LastSuccessAt           *time.Time
}

// CountersOf extracts the failure-streak state of an endpoint.
func CountersOf(endpoint *Endpoint) FailureCounters {
	return FailureCounters{
		ConsecutiveFailureCount: endpoint.ConsecutiveFailureCount,
		FirstFailureAt:          endpoint.FirstFailureAt,
		LastFailureAt:           endpoint.LastFailureAt,
		LastSuccessAt:           endpoint.LastSuccessAt,
	}
}

// RecordFailure advances the failure streak. The first failure of a streak
// stamps FirstFailureAt, which is what makes the age half of the auto-disable
// rule meaningful.
func RecordFailure(current FailureCounters, now time.Time) FailureCounters {
	next := current
	next.ConsecutiveFailureCount = current.ConsecutiveFailureCount + 1
	next.LastFailureAt = &now
	if current.FirstFailureAt == nil || current.ConsecutiveFailureCount == 0 {
		firstFailure := now
		next.FirstFailureAt = &firstFailure
	}

	return next
}

// RecordSuccess clears the failure streak. A single success resets both halves
// of the auto-disable rule, so a recovered endpoint starts from zero.
func RecordSuccess(current FailureCounters, now time.Time) FailureCounters {
	return FailureCounters{
		ConsecutiveFailureCount: 0,
		FirstFailureAt:          nil,
		LastFailureAt:           current.LastFailureAt,
		LastSuccessAt:           &now,
	}
}

// ShouldAutoDisable is the two-condition auto-disable rule: a long enough
// failure streak AND a streak that has been running long enough.
//
// Either condition alone is wrong. A ten-minute deploy blip can trivially
// produce twenty consecutive failures, and disabling a customer's integration
// over that is worse than the failures themselves; conversely a streak that is
// merely old but short is just a quiet endpoint.
func ShouldAutoDisable(counters FailureCounters, now time.Time) bool {
	if counters.ConsecutiveFailureCount < AutoDisableFailureThreshold {
		return false
	}
	if counters.FirstFailureAt == nil {
		return false
	}

	return now.Sub(*counters.FirstFailureAt) >= AutoDisableMinFailureAge
}

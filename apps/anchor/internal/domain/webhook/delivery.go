package webhook

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// DeliveryStatus is the lifecycle of one (event x endpoint) delivery.
//
// EXHAUSTED is the dead letter: a status rather than a separate table, so dead
// deliveries stay queryable, filterable and replayable next to their siblings.
type DeliveryStatus string

const (
	DeliveryStatusPending   DeliveryStatus = "PENDING"
	DeliveryStatusSucceeded DeliveryStatus = "SUCCEEDED"
	DeliveryStatusFailed    DeliveryStatus = "FAILED"
	DeliveryStatusExhausted DeliveryStatus = "EXHAUSTED"
)

func (s DeliveryStatus) IsValid() bool {
	switch s {
	case DeliveryStatusPending, DeliveryStatusSucceeded,
		DeliveryStatusFailed, DeliveryStatusExhausted:
		return true
	default:
		return false
	}
}

// IsTerminal reports whether the delivery will never be attempted again.
func (s DeliveryStatus) IsTerminal() bool {
	return s != DeliveryStatusPending
}

const (
	// MaxDeliveryAttempts is the length of the retry ladder.
	MaxDeliveryAttempts int32 = 8

	// MaxResponseReadBytes caps how much of a receiver's response body is read.
	MaxResponseReadBytes int64 = 64 * 1024

	// MaxResponseSnippetBytes caps how much of that body is persisted. A
	// hostile receiver could otherwise echo internal data into our admin UI.
	MaxResponseSnippetBytes = 2 * 1024

	// MaxErrorLength caps a persisted transport error string.
	MaxErrorLength = 1000
)

// Delivery is one (event x endpoint) pair with the exact bytes to send.
type Delivery struct {
	ID         string
	EventID    string
	EndpointID string
	ProductID  string
	Status     DeliveryStatus
	// AttemptCount is the number of attempts already made. It is the source of
	// truth for exhaustion, not the queue job's own counter.
	AttemptCount int32
	MaxAttempts  int32
	TargetURL    string
	// SignedBody is frozen at fan-out time and never re-marshalled: a deploy
	// that changed JSON field ordering would otherwise invalidate every
	// in-flight signature.
	SignedBody     string
	LastStatusCode *int32
	LastError      *string
	CompletedAt    *time.Time
	// ReplayOfDeliveryID points at the delivery this one manually replays.
	// Replays of replays are rejected in the service layer so the log keeps a
	// single hop back to the original.
	ReplayOfDeliveryID *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// GenerateID sets the delivery's ID to a new prefixed KSUID.
func (d *Delivery) GenerateID() {
	d.ID = ids.MustNew("whd")
}

// AttemptsRemaining reports whether another attempt is allowed.
func (d *Delivery) AttemptsRemaining() bool {
	return d.AttemptCount < d.MaxAttempts
}

// IsReplay reports whether this delivery was created by a manual retry.
func (d *Delivery) IsReplay() bool {
	return d.ReplayOfDeliveryID != nil
}

// DeliveryWithEvent pairs a delivery with the event that produced it, which is
// what the customer-visible delivery log shows.
type DeliveryWithEvent struct {
	Delivery Delivery
	Event    Event
}

// Attempt is one append-only row of the delivery log.
type Attempt struct {
	ID              string
	DeliveryID      string
	AttemptNumber   int32
	StatusCode      *int32
	Error           *string
	ResponseSnippet *string
	DurationMs      int32
	AttemptedAt     time.Time
	CreatedAt       time.Time
}

// GenerateID sets the attempt's ID to a new prefixed KSUID.
func (a *Attempt) GenerateID() {
	a.ID = ids.MustNew("wha")
}

// TruncateSnippet clips a response body to the persisted snippet size.
func TruncateSnippet(body []byte) string {
	if len(body) <= MaxResponseSnippetBytes {
		return string(body)
	}

	return string(body[:MaxResponseSnippetBytes])
}

// TruncateError clips a transport error message to the persisted size.
func TruncateError(message string) string {
	if len(message) <= MaxErrorLength {
		return message
	}

	return message[:MaxErrorLength]
}

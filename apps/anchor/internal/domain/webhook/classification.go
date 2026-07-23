package webhook

import (
	"net/http"
	"strconv"
	"time"
)

// Outcome is what an attempt's result means for the delivery and its endpoint.
type Outcome string

const (
	// OutcomeSucceeded means the receiver acknowledged the delivery.
	OutcomeSucceeded Outcome = "SUCCEEDED"
	// OutcomeRetry means a transient condition; try again on the ladder.
	OutcomeRetry Outcome = "RETRY"
	// OutcomeFailed means a permanent condition. A 404 will still be a 404 in
	// twelve hours, so burning seven more attempts on it helps nobody.
	OutcomeFailed Outcome = "FAILED"
	// OutcomeDisableEndpoint means the receiver declared the endpoint gone.
	OutcomeDisableEndpoint Outcome = "DISABLE_ENDPOINT"
)

const (
	statusClientErrorFloor = 400
	statusServerErrorFloor = 500
	statusSuccessFloor     = 200
	statusSuccessCeiling   = 300
)

// MaxRetryAfter caps how far a receiver's Retry-After header may push the next
// attempt. Without a cap a hostile or broken receiver could park a delivery
// arbitrarily far into the future.
const MaxRetryAfter = 12 * time.Hour

// Classify maps an attempt's result to an outcome.
//
// A transport error (timeout, DNS failure, connection reset, transient TLS) is
// always retryable: we never got an answer, so we cannot know the request was
// rejected on its merits.
func Classify(statusCode int, transportErr error) Outcome {
	if transportErr != nil {
		return OutcomeRetry
	}

	switch {
	case statusCode >= statusSuccessFloor && statusCode < statusSuccessCeiling:
		return OutcomeSucceeded
	case statusCode == http.StatusGone:
		return OutcomeDisableEndpoint
	case statusCode == http.StatusRequestTimeout,
		statusCode == http.StatusTooManyRequests:
		return OutcomeRetry
	case statusCode >= statusServerErrorFloor:
		return OutcomeRetry
	case statusCode >= statusClientErrorFloor:
		return OutcomeFailed
	default:
		// 1xx and 3xx. Redirects are never followed — a 302 to 127.0.0.1 is the
		// oldest SSRF trick there is — so a redirect response is a permanent
		// misconfiguration of the endpoint, not a transient condition.
		return OutcomeFailed
	}
}

// ParseRetryAfter interprets a Retry-After header, in either the delta-seconds
// or the HTTP-date form, and clamps the result to MaxRetryAfter. It returns nil
// when the header is absent or unparseable, in which case the caller falls back
// to the standard ladder.
func ParseRetryAfter(header string, now time.Time) *time.Duration {
	if header == "" {
		return nil
	}

	if seconds, err := strconv.Atoi(header); err == nil {
		return clampRetryAfter(time.Duration(seconds) * time.Second)
	}

	if at, err := http.ParseTime(header); err == nil {
		return clampRetryAfter(at.Sub(now))
	}

	return nil
}

func clampRetryAfter(delay time.Duration) *time.Duration {
	if delay < 0 {
		delay = 0
	}
	if delay > MaxRetryAfter {
		delay = MaxRetryAfter
	}

	return &delay
}

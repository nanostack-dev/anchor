package service

import (
	"errors"
	"time"
)

// RetryAfterError is the retryable error a delivery attempt returns when the
// receiver asked, via a Retry-After header, to be left alone for a while.
//
// pgqueue decides the next attempt time from the error the handler returns, so
// the requested delay has to travel back out through it. The worker's RetryDelay
// func unwraps this and uses Delay in place of the jittered ladder; anything
// that does not carry one falls back to the ladder as before.
type RetryAfterError struct {
	Delay time.Duration
	Err   error
}

func (e *RetryAfterError) Error() string { return e.Err.Error() }

func (e *RetryAfterError) Unwrap() error { return e.Err }

// NewRetryAfterError wraps err with a receiver-requested delay. A nil delay
// returns err untouched, so callers can pass a parsed header straight through.
func NewRetryAfterError(delay *time.Duration, err error) error {
	if delay == nil {
		return err
	}

	return &RetryAfterError{Delay: *delay, Err: err}
}

// RetryAfterDelay reports the receiver-requested delay carried by err, if any.
func RetryAfterDelay(err error) (time.Duration, bool) {
	var retryAfter *RetryAfterError
	if errors.As(err, &retryAfter) {
		return retryAfter.Delay, true
	}

	return 0, false
}

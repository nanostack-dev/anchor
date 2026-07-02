// Package logx contains small logging helpers shared across Anchor services.
package logx

import (
	"context"
	"errors"
	"strings"

	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

// pgQueryCanceled is the PostgreSQL SQLSTATE raised when a running statement is
// aborted because its context was canceled ("canceling statement due to user
// request"). lib/pq surfaces this as a *pq.Error and does not wrap
// context.Canceled, so errors.Is cannot see the cancellation.
const pgQueryCanceled = "57014"

// IsContextError reports whether err is a context cancellation or deadline
// expiry. These signal that the caller (request, parent context) went away
// rather than a server-side fault.
//
// Cancellations reach us in three shapes and all three must be recognised:
//   - the bare context sentinels, or an error that wraps them with %w;
//   - a lib/pq *pq.Error with SQLSTATE 57014, emitted when the driver cancels
//     an in-flight query because its context was canceled;
//   - a go-jet error, which prefixes driver errors with "jet: " using a verb
//     that drops the wrapped error from the Unwrap chain, so errors.Is/As both
//     miss the underlying cancellation. For that case we fall back to matching
//     the canonical context-error messages.
func IsContextError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == pgQueryCanceled {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, context.Canceled.Error()) ||
		strings.Contains(msg, context.DeadlineExceeded.Error())
}

// EventForError returns a zerolog event at a severity appropriate for err.
//
// Context cancellation and deadline-exceeded errors are downgraded to Warn:
// they happen routinely when a client disconnects or an upstream request is
// cancelled, and are not actionable server-side faults. Logging them at Error
// pollutes error dashboards and trips error-rate alerts on benign traffic.
// Every other error keeps Error severity.
//
// The returned event still needs the error attached (e.g. via .Err(err)) and a
// terminating .Msg(...) call by the caller, mirroring normal zerolog usage.
func EventForError(logger *zerolog.Logger, err error) *zerolog.Event {
	if IsContextError(err) {
		return logger.Warn()
	}
	return logger.Error()
}

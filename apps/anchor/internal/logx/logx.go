// Package logx contains small logging helpers shared across Anchor services.
package logx

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
)

// IsContextError reports whether err is a context cancellation or deadline
// expiry. These signal that the caller (request, parent context) went away
// rather than a server-side fault.
func IsContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
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

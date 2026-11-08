package applog

import (
	"context"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/pgerr"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

// IsContextError reports whether err is a cancellation or a deadline expiry,
// including lib/pq's "canceling statement due to user request" (SQLSTATE
// 57014), which does not wrap context.Canceled. core/log.IsContextError does
// not know the driver error.
func IsContextError(err error) bool {
	return log.IsContextError(err) || pgerr.IsQueryCanceled(err)
}

// LevelFor picks the level of an error line as pkg/log.LevelFor did: a
// cancellation is Warn, a caller fault (a pkg/fault error below 500, an
// invalid search cursor or sort) is Debug, and anything else is Error.
//
// core/log.LevelFor does not classify pkg/search errors or the driver's
// cancelled statement, so it would log every INVALID_CURSOR answer at Error.
func LevelFor(err error) slog.Level {
	switch {
	case err == nil:
		return slog.LevelError
	case IsContextError(err):
		return slog.LevelWarn
	case isClientFault(err):
		return slog.LevelDebug
	case search.IsClientError(err):
		return slog.LevelDebug
	default:
		return slog.LevelError
	}
}

func isClientFault(err error) bool {
	apiErr, ok := fault.As(err)
	return ok && apiErr != nil && apiErr.HTTPStatus() < http.StatusInternalServerError
}

// Error logs err at the level LevelFor picks, with the error and, when err is
// a pkg/fault error, its HTTP status, as pkg/log.Event did. The caller is the
// line that calls Error.
func Error(ctx context.Context, logger *slog.Logger, err error, msg string, attrs ...slog.Attr) {
	level := LevelFor(err)
	if !logger.Enabled(ctx, level) {
		return
	}
	all := make([]slog.Attr, 0, len(attrs)+2) //nolint:mnd // the error and its status
	all = append(all, log.Err(err))
	if apiErr, ok := fault.As(err); ok && apiErr != nil {
		all = append(all, slog.Int(log.KeyStatus, apiErr.HTTPStatus()))
	}
	all = append(all, attrs...)
	Write(ctx, logger, 1, level, msg, all...)
}

// Write writes one record whose caller is skip frames above the function that
// calls Write: 0 is that function itself. Helpers that log on behalf of their
// caller use it so the caller field names the call site, as zerolog did.
func Write(ctx context.Context, logger *slog.Logger, skip int, level slog.Level, msg string, attrs ...slog.Attr) {
	if !logger.Enabled(ctx, level) {
		return
	}
	const callersSkip = 2 // runtime.Callers and Write
	var pcs [1]uintptr
	runtime.Callers(callersSkip+skip, pcs[:])
	record := slog.NewRecord(time.Now(), level, msg, pcs[0])
	record.AddAttrs(attrs...)
	_ = logger.Handler().Handle(ctx, record) // like slog.Logger, a failed write is not the caller's error
}

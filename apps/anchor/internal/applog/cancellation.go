package applog

import (
	"context"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"
)

// cancellationHandler writes an error line whose error is a cancellation at
// warn, the level core/log's log.LevelFor gives it: the caller went away, the
// service did not fail.
//
// log.ErrorTo already picks that level, but a call site that logs with
// logger.Error or logger.ErrorContext and log.Err(err) fixes the level at
// error. Two per-site fixes (anchor#320, anchor#334) left such sites behind,
// and a client that hung up logged at error in prod again
// (role assignment and member search, 2026-11-25T04:49:50Z). The handler
// applies the rule to every line, so a new site cannot miss it.
//
// It reads the error under log.KeyError on the record and on the logger's
// With attributes. Only the cancellation rule applies here: the 4xx and
// client-error rules stay with log.ErrorTo, because a raw site may log a
// fault it means to report.
type cancellationHandler struct {
	inner slog.Handler
	// cancelled is true when the With attributes carry a cancellation under
	// log.KeyError.
	cancelled bool
}

func (h cancellationHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h cancellationHandler) Handle(ctx context.Context, r slog.Record) error {
	cancelled := r.Level == slog.LevelError && (h.cancelled || recordCancelled(r))
	if !cancelled {
		return h.inner.Handle(ctx, r)
	}
	if !h.inner.Enabled(ctx, slog.LevelWarn) {
		return nil
	}
	r.Level = slog.LevelWarn
	return h.inner.Handle(ctx, r)
}

func (h cancellationHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cancelled := h.cancelled
	for _, a := range attrs {
		cancelled = cancelled || isCancellation(a)
	}
	return cancellationHandler{inner: h.inner.WithAttrs(attrs), cancelled: cancelled}
}

func (h cancellationHandler) WithGroup(name string) slog.Handler {
	return cancellationHandler{inner: h.inner.WithGroup(name), cancelled: h.cancelled}
}

func recordCancelled(r slog.Record) bool {
	cancelled := false
	r.Attrs(func(a slog.Attr) bool {
		cancelled = isCancellation(a)
		return !cancelled
	})
	return cancelled
}

func isCancellation(a slog.Attr) bool {
	if a.Key != log.KeyError {
		return false
	}
	err, ok := a.Value.Resolve().Any().(error)
	return ok && log.IsContextError(err)
}

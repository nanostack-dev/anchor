package applog

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

// Write writes one record whose caller is skip frames above the function that
// calls Write: 0 is that function itself. Helpers that log on behalf of their
// caller use it so the caller field names the call site, as zerolog did.
//
// An error line uses core/log's log.ErrorTo instead, which picks the level
// from the error and names its own caller.
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

package service

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	"anchor/internal/applog"
)

// logServiceError logs err with msg: a pkg/fault error below 500 at Debug
// and any other error at Error, with the fault's HTTP status when there is
// one. The caller field names the line that calls it.
func logServiceError(ctx context.Context, logger *slog.Logger, err error, msg string, attrs ...slog.Attr) {
	level := slog.LevelError
	all := make([]slog.Attr, 0, len(attrs)+2) //nolint:mnd // the error and its status
	all = append(all, log.Err(err))
	if apiErr, ok := fault.As(err); ok && apiErr != nil {
		status := apiErr.HTTPStatus()
		if status < http.StatusInternalServerError {
			level = slog.LevelDebug
		}
		all = append(all, slog.Int(log.KeyStatus, status))
	}
	applog.Write(ctx, logger, 1, level, msg, append(all, attrs...)...)
}

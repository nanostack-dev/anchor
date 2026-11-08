package middleware

import (
	"fmt"
	"log/slog"
	"net/http"

	"anchor/internal/applog"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/httputil/errorenvelope"
)

// ErrorMiddleware is the strict server's request and response error hooks.
// pkg/httputil/errorenvelope resolves each error to the legacy
// {"errors":[...]} envelope and writes it, as the deprecated
// modules/httpserver.StrictErrorHandler did.
//
// The log lines are anchor's: errorenvelope.Writer logs "request failed" at
// Debug for a 4xx and also logs request-hook errors, while the golden suite
// and the alerts read StrictErrorHandler's lines ("Request error" at Info for
// a handled 4xx, Error lines with error_type and error_detail for a 5xx, and
// nothing for a request-hook error).
type ErrorMiddleware struct {
	logger *slog.Logger
}

// NewErrorMiddleware creates a new ErrorMiddleware instance.
func NewErrorMiddleware(logger applog.Scoped) *ErrorMiddleware {
	return &ErrorMiddleware{logger: logger.Logger}
}

// HandleRequestError handles request decode and binding failures (a malformed
// body). It logs nothing: the request summary line records the 400.
func (em *ErrorMiddleware) HandleRequestError(w http.ResponseWriter, r *http.Request, err error) {
	errorenvelope.ResolveRequest(r, err, errorenvelope.Options{}).Write(w)
}

// HandleResponseError handles errors returned by API handlers: a handled
// fault below 500 logs at Info, a 5xx or an unmodelled error at Error, and a
// request the client abandoned at Warn.
func (em *ErrorMiddleware) HandleResponseError(w http.ResponseWriter, r *http.Request, err error) {
	resp := errorenvelope.Resolve(r, err, errorenvelope.Options{})
	ctx := r.Context()
	status := slog.Int(log.KeyStatus, resp.Status)
	switch {
	case resp.Abandoned:
		applog.Write(ctx, em.logger, 0, slog.LevelWarn, "Request abandoned by client", log.Err(err), status)
	case resp.Unmodelled:
		em.logInternalError(r, err, status, "Unhandled error returned by strict handler")
	case resp.Status >= http.StatusInternalServerError:
		em.logInternalError(r, err, status, "Internal server error")
	default:
		applog.Write(ctx, em.logger, 0, slog.LevelInfo, "Request error", log.Err(err), status)
	}
	resp.Write(w)
}

func (em *ErrorMiddleware) logInternalError(r *http.Request, err error, status slog.Attr, msg string) {
	applog.Write(r.Context(), em.logger, 1, slog.LevelError, msg,
		log.Err(err),
		slog.String(errorenvelope.KeyErrorType, fmt.Sprintf("%T", err)),
		slog.String(errorenvelope.KeyErrorDetail, fmt.Sprintf("%+v", err)),
		status,
	)
}

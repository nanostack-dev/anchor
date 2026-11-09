package middleware

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/httputil/errorenvelope"

	"anchor/internal/applog"
)

// NewErrorEnvelope returns the strict server's request and response error
// hooks (RequestError and ResponseError). pkg/httputil/errorenvelope resolves
// each error to the legacy {"errors":[...]} envelope and writes it, as the
// deprecated modules/httpserver.StrictErrorHandler did.
//
// The lines are StrictErrorHandler's, which the golden suite and the alerts
// read: "Request error" at Info for a handled 4xx, Error lines with
// error_type and error_detail for a 5xx, Warn for a request the client
// abandoned, and nothing for a request-hook error.
func NewErrorEnvelope(logger applog.Scoped) *errorenvelope.Writer {
	return errorenvelope.New(errorenvelope.Options{Log: errorenvelope.StrictErrorHandlerLog(logger.Logger)})
}

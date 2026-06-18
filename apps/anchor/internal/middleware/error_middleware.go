package middleware

import (
	"net/http"

	frameworkhttpserver "github.com/nanostack-dev/nanostack-framework/modules/httpserver"
	"github.com/rs/zerolog"
)

// ErrorMiddleware adapts the framework strict error handler to anchor's
// generated RequestErrorHandlerFunc/ResponseErrorHandlerFunc hooks. The handler
// writes the canonical fault error contract and applies the standard boundary
// log severities (handled <500 at info, 5xx and unmodelled at error).
type ErrorMiddleware struct {
	handler *frameworkhttpserver.StrictErrorHandler
}

// NewErrorMiddleware creates a new ErrorMiddleware instance.
func NewErrorMiddleware(logger zerolog.Logger) *ErrorMiddleware {
	return &ErrorMiddleware{
		handler: frameworkhttpserver.NewStrictErrorHandler(frameworkhttpserver.StrictErrorHandlerOptions{
			Logger: logger,
		}),
	}
}

// HandleRequestError handles request decode/binding failures (malformed body).
func (em *ErrorMiddleware) HandleRequestError(w http.ResponseWriter, r *http.Request, err error) {
	em.handler.HandleRequestError(w, r, err)
}

// HandleResponseError handles errors returned by API handlers.
func (em *ErrorMiddleware) HandleResponseError(w http.ResponseWriter, r *http.Request, err error) {
	em.handler.HandleResponseError(w, r, err)
}

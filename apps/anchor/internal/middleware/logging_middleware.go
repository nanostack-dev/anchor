package middleware

import (
	"net/http"

	sharedMiddleware "github.com/nanostack-dev/shared/middlewares"
	"github.com/rs/zerolog"
)

const healthRoutePath = "/health"

// NewRequestLoggingMiddleware wraps the shared request logger and skips high-frequency
// endpoints that add noise to logs.
func NewRequestLoggingMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	baseLoggingMiddleware := sharedMiddleware.NewLoggingMiddleware(logger)

	return func(next http.Handler) http.Handler {
		loggedHandler := baseLoggingMiddleware(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == healthRoutePath {
				next.ServeHTTP(w, r)
				return
			}

			loggedHandler.ServeHTTP(w, r)
		})
	}
}

package middleware

import (
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/httputil/requestlog"
	"github.com/rs/zerolog"
)

const healthRoutePath = "/health"

// NewRequestLoggingMiddleware wraps the framework request logger and skips high-frequency
// endpoints that add noise to logs.
func NewRequestLoggingMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	baseLoggingMiddleware := requestlog.NewFromEnv(logger)

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

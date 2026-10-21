package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/httputil/requestlog"
	"github.com/rs/zerolog"
)

const healthRoutePath = "/health"

// credentialBodySuffixes name the routes whose request body carries a raw
// organization API key. Their bodies are never logged.
var credentialBodySuffixes = []string{ //nolint:gochecknoglobals // a fixed route list
	"/access/v1/evaluation",
	"/access/v1/evaluations",
	"/auth/introspect",
	"/api-keys/validate",
}

// NewRequestLoggingMiddleware wraps the framework request logger and skips high-frequency
// endpoints that add noise to logs.
func NewRequestLoggingMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return newRequestLoggingMiddleware(logger, os.Getenv("LOG_REQUEST_BODY") == "true")
}

func newRequestLoggingMiddleware(logger zerolog.Logger, logBodies bool) func(http.Handler) http.Handler {
	baseLoggingMiddleware := requestlog.New(logger, requestlog.Options{LogRequestBody: logBodies})
	withoutBodyMiddleware := requestlog.New(logger, requestlog.Options{LogRequestBody: false})

	return func(next http.Handler) http.Handler {
		loggedHandler := baseLoggingMiddleware(next)
		loggedWithoutBody := withoutBodyMiddleware(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == healthRoutePath {
				next.ServeHTTP(w, r)
				return
			}
			if carriesCredentialBody(r.URL.Path) {
				loggedWithoutBody.ServeHTTP(w, r)
				return
			}

			loggedHandler.ServeHTTP(w, r)
		})
	}
}

func carriesCredentialBody(path string) bool {
	for _, suffix := range credentialBodySuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

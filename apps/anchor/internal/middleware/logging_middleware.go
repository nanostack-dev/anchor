package middleware

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"anchor/internal/applog"

	"github.com/nanostack-dev/nanostack-framework/core/log"
)

const (
	healthRoutePath = "/health"
	// maxLoggedBodyBytes caps a logged request body, as the zerolog request
	// log did.
	maxLoggedBodyBytes = 4 * 1024
	bodyReadFailed     = "Error reading request body"
)

// credentialBodySuffixes name the routes whose request body carries a raw
// organization API key. Their bodies are never logged.
var credentialBodySuffixes = []string{ //nolint:gochecknoglobals // a fixed route list
	"/access/v1/evaluation",
	"/access/v1/evaluations",
	"/auth/introspect",
	"/api-keys/validate",
}

// NewRequestLoggingMiddleware writes one summary line per request, with
// LOG_REQUEST_BODY=true adding the request body.
func NewRequestLoggingMiddleware(logger applog.Scoped) func(http.Handler) http.Handler {
	return newRequestLoggingMiddleware(logger.Logger, os.Getenv("LOG_REQUEST_BODY") == "true")
}

// newRequestLoggingMiddleware writes the summary line of the zerolog request
// log (pkg/httputil/requestlog) on slog, which the golden suite, the alerts
// and the dashboards read: the message "<METHOD> <path> -> <status>
// (<duration>)", the caller, status, bytes_out, duration in milliseconds,
// client_ip (the first X-Forwarded-For hop), query, user_agent and, when
// logBodies is set, request_body. It is Error for a 5xx and Info otherwise.
// request_id, method and path come from the request scope, and the caller
// from the attributes auth adds to it.
//
// core/log.HTTPMiddleware writes a different line ("http request", no caller
// and no body), and it runs outside auth, where it would log the auth
// refusals that this line does not log today.
func newRequestLoggingMiddleware(logger *slog.Logger, logBodies bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == healthRoutePath {
				next.ServeHTTP(w, r)
				return
			}
			withBody := logBodies && !carriesCredentialBody(r.URL.Path)
			start := time.Now()
			body := requestBodyForLogging(logger, r, withBody)
			recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(recorder, r)
			logRequest(logger, r, recorder, start, body)
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

func requestBodyForLogging(logger *slog.Logger, r *http.Request, withBody bool) string {
	if !withBody || r.Body == nil {
		return ""
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.ErrorContext(r.Context(), "failed to read request body", log.Err(err))
		return bodyReadFailed
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	body := string(bodyBytes)
	if len(body) > maxLoggedBodyBytes {
		body = body[:maxLoggedBodyBytes] + "...(truncated)"
	}
	return body
}

func logRequest(logger *slog.Logger, r *http.Request, recorder *statusRecorder, start time.Time, body string) {
	duration := time.Since(start)
	status := recorder.statusCode

	level := slog.LevelInfo
	if status >= http.StatusInternalServerError && status <= 599 {
		level = slog.LevelError
	}
	attrs := []slog.Attr{
		slog.Int(log.KeyStatus, status),
		slog.Int(log.KeyBytesOut, recorder.bytesWritten),
		slog.Duration(log.KeyDuration, duration),
		slog.String(log.KeyClientIP, clientIP(r)),
	}
	if query := r.URL.RawQuery; query != "" {
		attrs = append(attrs, slog.String(log.KeyQuery, query))
	}
	if userAgent := r.UserAgent(); userAgent != "" {
		attrs = append(attrs, slog.String(log.KeyUserAgent, userAgent))
	}
	if body != "" && body != bodyReadFailed {
		attrs = append(attrs, slog.String("request_body", body))
	}
	message := fmt.Sprintf("%s %s -> %d (%s)", r.Method, r.URL.Path, status, duration.Round(time.Microsecond))
	applog.Write(r.Context(), logger, 1, level, message, attrs...)
}

// clientIP is the first X-Forwarded-For hop, then X-Real-Ip, then the
// connection address, as the zerolog request log read it.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		return strings.TrimSpace(first)
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// statusRecorder records the status and the bytes written. It keeps
// http.Flusher and Unwrap, as the zerolog request log's recorder did.
type statusRecorder struct {
	http.ResponseWriter

	statusCode   int
	bytesWritten int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *statusRecorder) Write(b []byte) (int, error) {
	n, err := rec.ResponseWriter.Write(b)
	rec.bytesWritten += n
	return n, err //nolint:wrapcheck // a ResponseWriter passes its error through
}

func (rec *statusRecorder) Flush() {
	if flusher, ok := rec.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rec *statusRecorder) Unwrap() http.ResponseWriter {
	return rec.ResponseWriter
}

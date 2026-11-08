package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/applog"
	"anchor/internal/middleware"
)

// errorHookLines serves one request whose handler hands err to hook, behind
// core/log.HTTPMiddleware as in cmd/http/server.go, and returns the response
// and the log lines.
func errorHookLines(
	t *testing.T, err error,
	hook func(*middleware.ErrorMiddleware) func(http.ResponseWriter, *http.Request, error),
) (*httptest.ResponseRecorder, []map[string]any) {
	t.Helper()
	var buffer bytes.Buffer
	loggers := applog.New(log.Config{Wire: log.WireLegacy, Output: &buffer, Level: slog.LevelDebug})
	handle := hook(middleware.NewErrorMiddleware(loggers.Scoped))
	handler := log.HTTPMiddleware(log.HTTPOptions{
		Logger:      loggers.Scoped.Logger,
		SkipSummary: func(*http.Request) bool { return true },
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handle(w, r, err) }))

	request := httptest.NewRequest(http.MethodGet, "/v1/products/prd_1", nil)
	request.Header.Set(log.RequestIDHeader, "req_hook")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	var lines []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buffer.String()), "\n") {
		if line == "" {
			continue
		}
		var fields map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &fields), line)
		lines = append(lines, fields)
	}
	return recorder, lines
}

func responseHook(em *middleware.ErrorMiddleware) func(http.ResponseWriter, *http.Request, error) {
	return em.HandleResponseError
}

// The response hook writes the lines of the deprecated
// modules/httpserver.StrictErrorHandler, which the 5xx log alert and the
// support runbooks read. The golden suite pins only the 4xx line: it cannot
// make the server fail, so the 5xx and abandoned branches are pinned here.
func TestResponseErrorLinesKeepTheStrictErrorHandlerShape(t *testing.T) {
	unmodelled := errors.New("pq: connection refused")
	cases := []struct {
		name       string
		err        error
		status     int
		level      string
		message    string
		diagnostic bool // error_type and error_detail, on the 5xx lines only
	}{
		{
			name:    "a handled fault below 500 is an info line",
			err:     fault.NewWithStatus("PRODUCT_NOT_FOUND", "Product not found", http.StatusNotFound),
			status:  http.StatusNotFound,
			level:   "info",
			message: "Request error",
		},
		{
			name:       "a modelled 5xx is an error line with the diagnostics",
			err:        fmt.Errorf("load product: %w", fault.ErrUnexpected),
			status:     http.StatusInternalServerError,
			level:      "error",
			message:    "Internal server error",
			diagnostic: true,
		},
		{
			name:       "an unmodelled error is an error line with the diagnostics",
			err:        unmodelled,
			status:     http.StatusInternalServerError,
			level:      "error",
			message:    "Unhandled error returned by strict handler",
			diagnostic: true,
		},
		{
			name:    "a request the client abandoned is a warn line with 499",
			err:     context.Canceled,
			status:  499,
			level:   "warn",
			message: "Request abandoned by client",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder, lines := errorHookLines(t, tc.err, responseHook)

			assert.Equal(t, tc.status, recorder.Code)
			require.Len(t, lines, 1)
			line := lines[0]
			assert.Equal(t, tc.level, line["level"])
			assert.Equal(t, tc.message, line["message"])
			assert.Equal(t, tc.err.Error(), line["error"])
			assert.InDelta(t, tc.status, line["status"], 0)
			assert.Equal(t, "req_hook", line["request_id"])
			assert.Equal(t, http.MethodGet, line["method"])
			assert.Equal(t, "/v1/products/prd_1", line["path"])
			assert.Regexp(t, `/error_middleware\.go:\d+$`, line["caller"], "the caller is the hook, as zerolog's was")
			if tc.diagnostic {
				assert.Equal(t, fmt.Sprintf("%T", tc.err), line["error_type"])
				assert.Equal(t, fmt.Sprintf("%+v", tc.err), line["error_detail"])
			} else {
				assert.NotContains(t, line, "error_type")
				assert.NotContains(t, line, "error_detail")
			}
		})
	}
}

// A 5xx answers the generic UNEXPECTED_ERROR envelope: the cause stays in the
// log line, never on the wire.
func TestResponseErrorKeepsTheCauseOffTheWire(t *testing.T) {
	recorder, _ := errorHookLines(t, errors.New("pq: password authentication failed"), responseHook)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Body.String(), `"code":"UNEXPECTED_ERROR"`)
	assert.NotContains(t, recorder.Body.String(), "pq:")
}

// A request decoding error answers 400 and logs nothing, as
// StrictErrorHandler.HandleRequestError did: the summary line records it.
func TestRequestErrorWritesNoLine(t *testing.T) {
	recorder, lines := errorHookLines(t, errors.New("can't decode JSON body"),
		func(em *middleware.ErrorMiddleware) func(http.ResponseWriter, *http.Request, error) {
			return em.HandleRequestError
		})

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Empty(t, lines)
}

package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"anchor/internal/applog"
	"anchor/internal/middleware"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestLoggingKeepsRawKeysOutOfLogs(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		logged bool
	}{
		{"access evaluation hides the body", "/v1/products/prd_1/access/v1/evaluations", false},
		{"single access evaluation hides the body", "/v1/products/prd_1/access/v1/evaluation", false},
		{"introspection hides the body", "/v1/products/prd_1/auth/introspect", false},
		{"key validation hides the body", "/v1/products/prd_1/organizations/org_1/api-keys/validate", false},
		{"another route still logs its body", "/v1/products/prd_1/organizations", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buffer bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buffer, nil))
			logged := middleware.NewRequestLoggingMiddlewareWithBodies(logger, true)(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
			)
			request := httptest.NewRequest(http.MethodPost, tc.path,
				strings.NewReader(`{"subject":{"type":"organization_api_key","id":"anchor_org_apikey_secret"}}`))

			logged.ServeHTTP(httptest.NewRecorder(), request)

			assert.NotEmpty(t, buffer.String(), "the request is still logged")
			assert.Equal(t, tc.logged, strings.Contains(buffer.String(), "anchor_org_apikey_secret"))
		})
	}
}

// The summary line keeps the zerolog request log's shape, which the alerts
// and the golden suite read.
func TestRequestLoggingWritesTheLegacySummaryLine(t *testing.T) {
	var buffer bytes.Buffer
	logger := applog.New(log.Config{Wire: log.WireLegacy, Output: &buffer}).Scoped.Logger
	logged := middleware.NewRequestLoggingMiddlewareWithBodies(logger, true)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("down"))
		}),
	)
	request := httptest.NewRequest(http.MethodPost, "/v1/products?page=2", strings.NewReader(`{"name":"x"}`))
	request.Header.Set("User-Agent", "probe/1")
	request.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")

	logged.ServeHTTP(httptest.NewRecorder(), request)

	var line map[string]any
	require.NoError(t, json.Unmarshal(buffer.Bytes(), &line))
	assert.Regexp(t, `^POST /v1/products -> 502 \(.+\)$`, line["message"])
	assert.Equal(t, "error", line["level"], "a 5xx summary is an error line")
	assert.Regexp(t, `/logging_middleware\.go:\d+$`, line["caller"])
	assert.InDelta(t, 502, line["status"], 0)
	assert.InDelta(t, 4, line["bytes_out"], 0)
	assert.Equal(t, "page=2", line["query"])
	assert.Equal(t, "probe/1", line["user_agent"])
	assert.Equal(t, "203.0.113.7", line["client_ip"], "the first X-Forwarded-For hop")
	assert.JSONEq(t, `{"name":"x"}`, line["request_body"].(string))
	assert.IsType(t, float64(0), line["duration"])
}

package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"anchor/internal/middleware"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
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
			logged := middleware.NewRequestLoggingMiddlewareWithBodies(zerolog.New(&buffer), true)(
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

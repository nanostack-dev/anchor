package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"anchor/internal/api"
)

func TestWorkspaceDeprecationMiddleware(t *testing.T) {
	const base = "/v1/products/prd_1/organizations/org_1"
	for name, tc := range map[string]struct {
		path string
		link string
	}{
		"The collection links to the resource search": {
			path: base + "/workspaces", link: "<" + base + `/resources/search>; rel="successor-version"`,
		},
		"The search links to the resource search": {
			path: base + "/workspaces/search", link: "<" + base + `/resources/search>; rel="successor-version"`,
		},
		"One workspace links to its resource": {
			path: base + "/workspaces/ws_1", link: "<" + base + `/resources/workspace/ws_1>; rel="successor-version"`,
		},
		"Another route gets no header":            {path: base + "/resources/workspace/ws_1"},
		"A path below a workspace gets no header": {path: base + "/workspaces/ws_1/members"},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler := api.WorkspaceDeprecationMiddleware(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			)

			handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil))

			assert.Equal(t, tc.link, recorder.Header().Get("Link"))
			if tc.link == "" {
				assert.Empty(t, recorder.Header().Get("Deprecation"))
			} else {
				assert.Equal(t, "@1792886400", recorder.Header().Get("Deprecation"))
			}
		})
	}
}

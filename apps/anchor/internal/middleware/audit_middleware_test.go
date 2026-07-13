package middleware_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"anchor/internal/middleware"
)

func TestDeriveRouteAction(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		pattern    string
		wantAction string
		wantTarget string
	}{
		{
			"create on collection",
			http.MethodPost, "/v1/products/{product_id}/organizations",
			"organizations.created", "organizations",
		},
		{
			"delete with trailing param",
			http.MethodDelete,
			"/v1/products/{product_id}/organizations/{organization_id}/api-keys/{api_key_id}",
			"api_keys.deleted", "api_keys",
		},
		{
			"put maps to updated",
			http.MethodPut, "/v1/products/{product_id}/organizations/{organization_id}",
			"organizations.updated", "organizations",
		},
		{
			"patch maps to updated",
			http.MethodPatch, "/v1/products/{product_id}/roles/{role_id}",
			"roles.updated", "roles",
		},
		{
			"dashes become underscores",
			http.MethodPost, "/v1/products/{product_id}/resource-permissions",
			"resource_permissions.created", "resource_permissions",
		},
		{
			"no static segment",
			http.MethodPost, "/v1",
			"", "",
		},
		{
			"unsupported method",
			http.MethodGet, "/v1/products/{product_id}/organizations",
			"", "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			action, target := middleware.DeriveRouteAction(tc.method, tc.pattern)
			if action != tc.wantAction || target != tc.wantTarget {
				t.Fatalf(
					"middleware.DeriveRouteAction(%s, %s) = (%q, %q), want (%q, %q)",
					tc.method, tc.pattern, action, target, tc.wantAction, tc.wantTarget,
				)
			}
		})
	}
}

func TestIsSkippedPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/v1/products/prd_x/audit-logs/search", true},
		{"/v1/products/prd_x/organizations/org_x/api-keys/validate", true},
		{"/v1/products/prd_x/auth/introspect", true},
		{"/v1/products/prd_x/email/templates/tpl_x/preview", true},
		{"/v1/products/prd_x/email/sends", true},
		{"/v1/products/prd_x/organizations", false},
		{"/v1/products/prd_x/api-keys", false},
	}
	for _, tc := range tests {
		if got := middleware.IsSkippedPath(tc.path); got != tc.want {
			t.Errorf("middleware.IsSkippedPath(%s) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestLastPathParamValue(t *testing.T) {
	tests := []struct {
		name   string
		keys   []string
		values []string
		want   string
	}{
		{"trailing resource param", []string{"product_id", "api_key_id"}, []string{"prd_1", "key_1"}, "key_1"},
		{"scoping param last", []string{"product_id", "organization_id"}, []string{"prd_1", "org_1"}, ""},
		{"product only", []string{"product_id"}, []string{"prd_1"}, ""},
		{"no params", nil, nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			routeCtx := chi.NewRouteContext()
			for i, key := range tc.keys {
				routeCtx.URLParams.Add(key, tc.values[i])
			}
			if got := middleware.LastPathParamValue(routeCtx); got != tc.want {
				t.Fatalf("lastPathParamValue = %q, want %q", got, tc.want)
			}
		})
	}
}

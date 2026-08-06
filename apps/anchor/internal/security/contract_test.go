package security_test

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/nanostack-dev/nanostack-framework/pkg/apisec"

	httpserver "anchor/cmd/http"
)

// goldenPath holds the security posture of every documented operation.
const goldenPath = "testdata/route_security.json"

// TestContractSecurityIsUnchanged pins who may reach every operation.
//
// Authorisation is derived from the contract at runtime, so an edit to a
// `security:` block silently changes which credentials reach an endpoint —
// there is no generated code left whose diff would show it. This test resolves
// every operation the same way the auth middleware does and compares the result
// against a checked-in golden file, so widening or narrowing a route has to be
// an explicit, reviewable change to that file.
//
// Regenerate deliberately, never reflexively:
//
//	UPDATE_ROUTE_SECURITY=1 go test ./internal/security/ -run TestContractSecurityIsUnchanged
func TestContractSecurityIsUnchanged(t *testing.T) {
	t.Parallel()

	// The embedded document is what the server actually serves and resolves
	// against, so the test cannot drift from production by reading a copy.
	doc, err := openapi3.NewLoader().LoadFromData(httpserver.OpenAPI)
	if err != nil {
		t.Fatalf("load embedded spec: %v", err)
	}
	resolver, err := apisec.NewResolver(doc)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	actual := map[string]map[string][]string{}
	for _, route := range documentedRoutes(doc) {
		reqs, ok := resolver.For(httptest.NewRequest(route.method, route.concrete, nil))
		if !ok {
			t.Errorf("%s %s: the resolver matches no operation, so the auth "+
				"middleware would refuse every request to it", route.method, route.template)
			continue
		}

		schemes := map[string][]string{}
		for _, name := range reqs.Schemes() {
			scopes, _ := reqs.ScopesFor(name)
			if scopes == nil {
				scopes = []string{}
			}
			schemes[name] = scopes
		}
		if reqs.Public() {
			// Recorded explicitly: an operation becoming reachable without
			// credentials is the change most worth catching.
			schemes["<public>"] = []string{}
		}
		actual[route.method+" "+route.template] = schemes
	}

	if os.Getenv("UPDATE_ROUTE_SECURITY") != "" {
		writeGolden(t, actual)
		t.Logf("rewrote %s with %d operations", goldenPath, len(actual))
		return
	}

	var expected map[string]map[string][]string
	raw, readErr := os.ReadFile(goldenPath)
	if readErr != nil {
		t.Fatalf("read %s: %v (run with UPDATE_ROUTE_SECURITY=1 to create it)", goldenPath, readErr)
	}
	if parseErr := json.Unmarshal(raw, &expected); parseErr != nil {
		t.Fatalf("parse %s: %v", goldenPath, parseErr)
	}

	for _, op := range union(expected, actual) {
		want, inGolden := expected[op]
		got, inSpec := actual[op]
		switch {
		case !inGolden:
			t.Errorf("%s: new operation, security %v — add it to %s", op, got, goldenPath)
		case !inSpec:
			t.Errorf("%s: operation removed from the contract — drop it from %s", op, goldenPath)
		case !sameSchemes(want, got):
			t.Errorf("%s: security changed\n  golden: %v\n  spec:   %v", op, want, got)
		}
	}
}

type documentedRoute struct {
	method   string
	template string
	concrete string
}

func documentedRoutes(doc *openapi3.T) []documentedRoute {
	var out []documentedRoute
	for template, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			out = append(out, documentedRoute{
				method:   method,
				template: template,
				concrete: fillPathParams(template),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].template != out[j].template {
			return out[i].template < out[j].template
		}
		return out[i].method < out[j].method
	})
	return out
}

// fillPathParams substitutes a placeholder for each {param}. The value only has
// to route; it is never bound to a typed parameter here.
func fillPathParams(template string) string {
	segments := strings.Split(template, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			segments[i] = "placeholder"
		}
	}
	return strings.Join(segments, "/")
}

func writeGolden(t *testing.T, actual map[string]map[string][]string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(goldenPath), 0o750); err != nil {
		t.Fatalf("create testdata dir: %v", err)
	}
	blob, marshalErr := json.MarshalIndent(actual, "", "  ")
	if marshalErr != nil {
		t.Fatalf("marshal golden: %v", marshalErr)
	}
	if writeErr := os.WriteFile(goldenPath, append(blob, '\n'), 0o600); writeErr != nil {
		t.Fatalf("write golden: %v", writeErr)
	}
}

func sameSchemes(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for scheme, aScopes := range a {
		bScopes, ok := b[scheme]
		if !ok || len(aScopes) != len(bScopes) {
			return false
		}
		x := append([]string(nil), aScopes...)
		y := append([]string(nil), bScopes...)
		sort.Strings(x)
		sort.Strings(y)
		for i := range x {
			if x[i] != y[i] {
				return false
			}
		}
	}
	return true
}

func union(a, b map[string]map[string][]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range []map[string]map[string][]string{a, b} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

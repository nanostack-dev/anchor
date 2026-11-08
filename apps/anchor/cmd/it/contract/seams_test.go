//go:build contract

package contract_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// seamCase is one hand-written request against a framework seam: routing,
// request decoding, auth, validation, success shapes and the log lines each
// produces. The gate flows do not cover these.
type seamCase struct {
	name     string
	desc     string
	method   string
	path     string
	headers  map[string]string
	body     string
	hashBody bool // record the body as length + sha256 (large documents)
	// capture registers response JSON fields for later cases and the
	// normalizer, as "{{name}}".
	capture map[string]string
}

type seamRun struct {
	t     *testing.T
	norm  *normalizer
	dyn   *dynamicVars
	scope map[string]string
}

func newSeamRun(t *testing.T) *seamRun {
	t.Helper()
	dyn := newDynamicVars()
	norm := &normalizer{}
	norm.register(dyn.runID, "{{$runId}}")
	return &seamRun{t: t, norm: norm, dyn: dyn, scope: map[string]string{"runId": dyn.runID}}
}

// expand replaces {{name}} with captured values.
func (s *seamRun) expand(text string) string {
	for name, value := range s.scope {
		text = strings.ReplaceAll(text, "{{"+name+"}}", value)
	}
	return text
}

func (s *seamRun) do(c seamCase) *response {
	s.t.Helper()
	path := s.expand(c.path)
	body := s.expand(c.body)
	headers := make(map[string]string, len(c.headers))
	for k, v := range c.headers {
		headers[k] = s.expand(v)
	}
	requestID := "ct/seams/" + c.name
	resp, err := rawDo(s.t.Context(), c.method, suite.baseURL+path, headers, body, requestID)
	if err != nil {
		s.t.Fatalf("%s: %v", c.name, err)
	}
	for name, jsonPath := range c.capture {
		value, selErr := selectPath(jsonPath, resp.parsed)
		str, ok := value.(string)
		if selErr != nil || !ok {
			s.t.Fatalf("%s: capture %s (%s) from %d %s", c.name, name, jsonPath, resp.status, resp.body)
		}
		s.scope[name] = str
		s.norm.register(str, "{{"+name+"}}")
	}
	lines, err := suite.logs.linesFor(requestID)
	if err != nil {
		s.t.Fatalf("%s: %v", c.name, err)
	}
	recorded := *resp
	if c.hashBody {
		sum := sha256.Sum256(resp.body)
		recorded.body = fmt.Appendf(nil, "<%d bytes, sha256 %s>\n", len(resp.body), hex.EncodeToString(sum[:]))
	}
	reqBody := body
	const maxShownBody = 512
	if len(reqBody) > maxShownBody {
		reqBody = fmt.Sprintf("%s...<%d bytes>", reqBody[:maxShownBody], len(body))
	}
	ex := exchange{
		method: c.method, path: path, reqHeaders: headers, reqBody: reqBody,
		resp: &recorded, logs: lines, requestID: requestID, description: c.desc,
		bodyLen: len(resp.body),
	}
	checkGolden(s.t, filepath.Join("seams", c.name+".golden"), s.norm.render(ex))
	return resp
}

const (
	jsonCT = "application/json"
	ownerH = "owner"
	keyH   = "key"
)

func (s *seamRun) headers(kind string, extra ...string) map[string]string {
	h := map[string]string{"Content-Type": jsonCT}
	switch kind {
	case ownerH:
		h["Authorization"] = "Bearer " + suite.ownerToken
	case keyH:
		h["X-Product-Api-Key"] = "{{product_key}}"
	}
	for i := 0; i+1 < len(extra); i += 2 {
		if extra[i+1] == "" {
			delete(h, extra[i])
			continue
		}
		h[extra[i]] = extra[i+1]
	}
	return h
}

// TestFrameworkSeams pins the behaviour the framework owns rather than the
// domain: routing, method handling, health, CORS, docs, decoding leniency,
// validation envelopes, auth refusals, success shapes and their log lines.
//
//nolint:maintidx // one flat table of cases reads better than helpers
func TestFrameworkSeams(t *testing.T) {
	s := newSeamRun(t)
	big := strings.Repeat("x", 2<<20)
	cases := []seamCase{
		// Setup and success shapes.
		{name: "product-create", desc: "201 success shape and the owner's log enrichment",
			method: http.MethodPost, path: "/v1/products", headers: s.headers(ownerH),
			body:    `{"name":"contract-seams-{{runId}}","description":"golden contract suite"}`,
			capture: map[string]string{"product": "$.id"}},
		{name: "product-get", desc: "200 success shape", method: http.MethodGet,
			path: "/v1/products/{{product}}", headers: s.headers(ownerH)},
		{
			name:    "product-key-create",
			desc:    "201 with the one-time key value",
			method:  http.MethodPost,
			path:    "/v1/products/{{product}}/api-keys",
			headers: s.headers(ownerH),
			body:    `{"name":"contract-seams-key","mutable":false,"permissions":["organization:read","organization:create"]}`,
			capture: map[string]string{"product_key": "$.value", "product_key_id": "$.id"},
		},
		{name: "org-create-with-key", desc: "201 through a product API key: the key's log enrichment",
			method: http.MethodPost, path: "/v1/products/{{product}}/organizations", headers: s.headers(keyH),
			body: `{"name":"contract-seams-org-{{runId}}"}`, capture: map[string]string{"org": "$.id"}},
		{name: "org-search-with-key", desc: "200 list shape", method: http.MethodPost,
			path: "/v1/products/{{product}}/organizations/search", headers: s.headers(keyH), body: `{}`},

		// Routing.
		{name: "route-unknown-v1", desc: "no OpenAPI operation: the validator answers 404 text/plain before auth",
			method: http.MethodGet, path: "/v1/does-not-exist"},
		{name: "route-unknown-root", desc: "unknown path outside /v1", method: http.MethodGet, path: "/nope"},
		{name: "route-wrong-method", desc: "known path, undeclared method: 405 from the validator",
			method: http.MethodPatch, path: "/v1/products", headers: s.headers(ownerH), body: `{}`},
		{name: "route-trailing-slash", desc: "trailing slash is a different route",
			method: http.MethodGet, path: "/v1/products/{{product}}/", headers: s.headers(ownerH)},
		{name: "path-param-invalid", desc: "malformed path id: validator 400 text/plain, before auth (no token)",
			method: http.MethodGet, path: "/v1/products/not-an-id"},
		{name: "path-param-invalid-authed", desc: "malformed path id with a valid token",
			method: http.MethodGet, path: "/v1/products/not-an-id", headers: s.headers(ownerH)},
		{
			name:    "query-param-invalid",
			desc:    "query parameter of the wrong type",
			method:  http.MethodGet,
			path:    "/v1/products/{{product}}/email/templates?limit=abc",
			headers: s.headers(ownerH),
		},
		{name: "route-head-v1", desc: "HEAD on a GET operation", method: http.MethodHead,
			path: "/v1/products/{{product}}", headers: s.headers(ownerH)},
		{name: "route-options-no-preflight", desc: "OPTIONS without the CORS preflight headers",
			method: http.MethodOptions, path: "/v1/products", headers: s.headers(ownerH)},
		{name: "health-get", desc: "/health skips the validator and the request log",
			method: http.MethodGet, path: "/health"},
		{name: "health-head", desc: "HEAD on /health", method: http.MethodHead, path: "/health"},
		{name: "health-post", desc: "wrong method on /health: chi 405", method: http.MethodPost, path: "/health"},
		{name: "health-delete", desc: "wrong method on /health: chi 405", method: http.MethodDelete, path: "/health"},
		{name: "cors-preflight", desc: "CORS preflight from a browser origin", method: http.MethodOptions,
			path: "/v1/products", headers: map[string]string{
				"Origin": "https://app.example.test", "Access-Control-Request-Method": "POST",
				"Access-Control-Request-Headers": "authorization,content-type",
			}},
		{name: "cors-simple", desc: "CORS headers on an authenticated GET", method: http.MethodGet,
			path: "/v1/products/{{product}}", headers: s.headers(ownerH, "Origin", "https://app.example.test")},
		{name: "docs", desc: "API reference page", method: http.MethodGet, path: "/docs", hashBody: true},
		{name: "openapi", desc: "embedded OpenAPI document", method: http.MethodGet, path: "/openapi.yaml",
			hashBody: true},

		// Auth.
		{name: "auth-missing", desc: "no credential: 401 JSON envelope", method: http.MethodGet,
			path: "/v1/products/{{product}}"},
		{name: "auth-bad-bearer", desc: "garbage bearer token", method: http.MethodGet,
			path: "/v1/products/{{product}}", headers: s.headers("", "Authorization", "Bearer not-a-token")},
		{name: "auth-bad-scheme", desc: "Basic instead of Bearer", method: http.MethodGet,
			path: "/v1/products/{{product}}", headers: s.headers("", "Authorization", "Basic Zm9vOmJhcg==")},
		{name: "auth-unknown-product-key", desc: "unknown product API key", method: http.MethodPost,
			path:    "/v1/products/{{product}}/organizations/search",
			headers: s.headers("", "X-Product-Api-Key", "anchor_prd_apikey_doesnotexist"), body: `{}`},
		{
			name:    "auth-key-missing-scope",
			desc:    "product API key without the scope: 403 with required_scopes",
			method:  http.MethodDelete,
			path:    "/v1/products/{{product}}/organizations/{{org}}",
			headers: s.headers(keyH),
		},
		{name: "auth-key-on-owner-route", desc: "product API key on a route that takes a bearer token only",
			method: http.MethodGet, path: "/v1/me", headers: s.headers(keyH)},

		// Request decoding.
		{name: "decode-malformed-json", desc: "truncated JSON: decoder text reaches the client",
			method: http.MethodPost, path: "/v1/products", headers: s.headers(ownerH), body: `{"name":`},
		{name: "decode-empty-body", desc: "no body on a JSON operation", method: http.MethodPost,
			path: "/v1/products", headers: s.headers(ownerH)},
		{name: "decode-wrong-type", desc: "a number where a string is declared", method: http.MethodPost,
			path: "/v1/products", headers: s.headers(ownerH), body: `{"name":5}`},
		{name: "decode-array", desc: "a JSON array instead of an object", method: http.MethodPost,
			path: "/v1/products", headers: s.headers(ownerH), body: `[]`},
		{name: "decode-text-plain", desc: "valid JSON sent as text/plain is decoded anyway",
			method: http.MethodPost, path: "/v1/products", headers: s.headers(ownerH, "Content-Type", "text/plain"),
			body: `{"name":"contract-seams-textplain-{{runId}}"}`},
		{name: "decode-form-urlencoded", desc: "valid JSON sent as a form content type",
			method: http.MethodPost, path: "/v1/products",
			headers: s.headers(ownerH, "Content-Type", "application/x-www-form-urlencoded"),
			body:    `{"name":"contract-seams-form-{{runId}}"}`},
		{name: "decode-no-content-type", desc: "valid JSON without Content-Type", method: http.MethodPost,
			path: "/v1/products", headers: s.headers(ownerH, "Content-Type", ""),
			body: `{"name":"contract-seams-noct-{{runId}}"}`},
		{name: "decode-unknown-member", desc: "unknown members are ignored", method: http.MethodPost,
			path: "/v1/products", headers: s.headers(ownerH),
			body: `{"name":"contract-seams-unknown-{{runId}}","bogus":1}`},
		{name: "decode-case-variant", desc: "member names match case-insensitively", method: http.MethodPost,
			path: "/v1/products", headers: s.headers(ownerH), body: `{"NAME":"contract-seams-case-{{runId}}"}`},
		{name: "decode-duplicate-member", desc: "duplicate member: the last one wins", method: http.MethodPost,
			path: "/v1/products", headers: s.headers(ownerH),
			body: `{"name":"contract-seams-first-{{runId}}","name":"contract-seams-last-{{runId}}"}`},
		{name: "decode-trailing-data", desc: "data after the JSON value is ignored", method: http.MethodPost,
			path: "/v1/products", headers: s.headers(ownerH),
			body: `{"name":"contract-seams-trailing-{{runId}}"} trailing`},
		{name: "decode-oversized", desc: "a 2 MiB body is accepted (no size limit)", method: http.MethodPost,
			path: "/v1/products", headers: s.headers(ownerH),
			body: `{"name":"contract-seams-big-{{runId}}","padding":"` + big + `"}`},

		// Validation and domain errors.
		{name: "validation-empty-name", desc: "service validation: 400 VALIDATION_ERROR with metadata",
			method: http.MethodPost, path: "/v1/products", headers: s.headers(ownerH), body: `{"name":""}`},
		{name: "validation-sort", desc: "unknown sort field on a search", method: http.MethodPost,
			path: "/v1/products/search", headers: s.headers(ownerH), body: `{"sort_by":"bogus"}`},
		{name: "validation-cursor", desc: "malformed cursor", method: http.MethodPost,
			path: "/v1/products/search", headers: s.headers(ownerH),
			body: `{"pagination":{"cursor":"not-a-cursor"}}`},
		{name: "domain-not-found", desc: "unknown product: 404 envelope", method: http.MethodGet,
			path: "/v1/products/prd_000000000000000000000000000", headers: s.headers(ownerH)},
		{name: "domain-conflict", desc: "duplicate product name: 409 envelope", method: http.MethodPost,
			path: "/v1/products", headers: s.headers(ownerH), body: `{"name":"contract-seams-{{runId}}"}`},

		{name: "webhook-oversized", desc: "inbound webhook body over 1 MiB: 400 text/plain before the router",
			method: http.MethodPost, path: "/v1/products/{{product}}/integrations/webhooks/CLERK",
			headers: s.headers(""), body: `{"padding":"` + big + `"}`},

		// Teardown, also a 204 shape.
		{name: "product-delete", desc: "204 success shape", method: http.MethodDelete,
			path: "/v1/products/{{product}}", headers: s.headers(ownerH)},
		{name: "product-get-deleted", desc: "404 after delete", method: http.MethodGet,
			path: "/v1/products/{{product}}", headers: s.headers(ownerH)},
	}
	for _, c := range cases {
		s.do(c)
	}
}

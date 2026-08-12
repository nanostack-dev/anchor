package security_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
)

const (
	productAPIKeyHeader = "X-Product-Api-Key"
	bearerPrefix        = "Bearer "
)

type routeSecurityCase struct {
	OperationID          string
	Method               string
	Path                 string
	OpMap                map[string]any
	RequiresBody         bool
	RequiredQueryParams  map[string]string
	HasExplicitSecurity  bool
	IsPublic             bool
	RequiresBearer       bool
	AllowsAPIKey         bool
	RequiredAPIKeyScopes []string
}

type openAPIRawSpec struct {
	Paths      map[string]map[string]any `yaml:"paths"`
	Components map[string]any            `yaml:"components"`
}

type securityFixture struct {
	productContext  *itdsl.ProductContext
	bearerToken     string
	allAPIKeyScopes []string
	apiKeyCache     map[string]apiKeyCredential
}

type apiKeyCredential struct {
	ID    string
	Value string
}

func newSecurityFixture(t *testing.T) *securityFixture {
	t.Helper()

	return &securityFixture{
		productContext:  createTestProductContext(t),
		bearerToken:     testOwnerUser(t).AccessToken,
		allAPIKeyScopes: collectAllProductAPIKeyScopes(t),
		apiKeyCache:     map[string]apiKeyCredential{},
	}
}

func collectAllProductAPIKeyScopes(t *testing.T) []string {
	t.Helper()

	routes, _ := loadSecurityRouteCases(t)
	set := map[string]struct{}{}
	for _, route := range routes {
		for _, scope := range route.RequiredAPIKeyScopes {
			if scope == "" {
				continue
			}
			set[scope] = struct{}{}
		}
	}

	allScopes := make([]string, 0, len(set))
	for scope := range set {
		allScopes = append(allScopes, scope)
	}
	sort.Strings(allScopes)

	return allScopes
}

func (fx *securityFixture) getOrCreateAPIKey(t *testing.T, scopes []string) apiKeyCredential {
	t.Helper()

	normalized := normalizeScopes(scopes)
	cacheKey := strings.Join(normalized, ",")
	if cached, ok := fx.apiKeyCache[cacheKey]; ok {
		return cached
	}

	req := nanostackClient.CreateProductAPIKeyJSONRequestBody{
		Name:        "security-test-key-" + ids.MustNew("key"),
		Description: new("Security contract test key"),
		Permissions: normalized,
	}

	resp, err := testTenant(t).OwnerClient.CreateProductAPIKeyWithResponse(
		context.Background(),
		fx.productContext.ProductID,
		req,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode())
	require.NotNil(t, resp.JSON201)

	cred := apiKeyCredential{ID: resp.JSON201.Id, Value: resp.JSON201.Value}
	fx.apiKeyCache[cacheKey] = cred
	return cred
}

func normalizeScopes(scopes []string) []string {
	clone := append([]string(nil), scopes...)
	sort.Strings(clone)
	return clone
}

func readOpenAPISpec(t *testing.T) (openAPIRawSpec, map[string]any) {
	t.Helper()

	data, err := os.ReadFile("../../http/openapi.yaml")
	require.NoError(t, err)

	var spec openAPIRawSpec
	require.NoError(t, yaml.Unmarshal(data, &spec))

	var raw map[string]any
	require.NoError(t, yaml.Unmarshal(data, &raw))

	components := map[string]any{}
	if c, ok := raw["components"].(map[string]any); ok {
		components = c
	}

	return spec, components
}

func loadSecurityRouteCases(t *testing.T) ([]routeSecurityCase, map[string]any) {
	t.Helper()

	spec, components := readOpenAPISpec(t)
	paramDefs := map[string]any{}
	if p, ok := components["parameters"].(map[string]any); ok {
		paramDefs = p
	}

	routes := make([]routeSecurityCase, 0)
	for path, methods := range spec.Paths {
		for method, rawOp := range methods {
			opMap, ok := rawOp.(map[string]any)
			if !ok {
				continue
			}
			route := buildRouteSecurityCase(t, method, path, opMap, methods, components, paramDefs)
			routes = append(routes, route)
		}
	}

	sort.Slice(
		routes, func(i, j int) bool {
			if routes[i].Path == routes[j].Path {
				return routes[i].Method < routes[j].Method
			}
			return routes[i].Path < routes[j].Path
		},
	)

	return routes, components
}

func buildRouteSecurityCase(
	t *testing.T,
	method, path string,
	opMap map[string]any,
	pathMethods map[string]any,
	components, paramDefs map[string]any,
) routeSecurityCase {
	t.Helper()

	opID, _ := opMap["operationId"].(string)
	if opID == "" {
		opID = strings.ToLower(method) + " " + path
	}

	hasExplicitSecurity, isPublic, requiresBearer, allowsAPIKey, requiredAPIKeyScopes := parseSecurity(opMap)

	requiresBody := checkIfRequiresBody(opMap)
	_, requiredQueryParams := extractParameters(opMap, pathMethods, components, paramDefs)

	return routeSecurityCase{
		OperationID:          opID,
		Method:               strings.ToUpper(method),
		Path:                 path,
		OpMap:                opMap,
		RequiresBody:         requiresBody,
		RequiredQueryParams:  requiredQueryParams,
		HasExplicitSecurity:  hasExplicitSecurity,
		IsPublic:             isPublic,
		RequiresBearer:       requiresBearer,
		AllowsAPIKey:         allowsAPIKey,
		RequiredAPIKeyScopes: requiredAPIKeyScopes,
	}
}

func parseSecurity(opMap map[string]any) (bool, bool, bool, bool, []string) {
	rawSecurity, ok := opMap["security"]
	if !ok {
		return false, false, false, false, nil
	}

	securityEntries, ok := rawSecurity.([]any)
	if !ok {
		return true, false, false, false, nil
	}

	if len(securityEntries) == 0 {
		return true, true, false, false, nil
	}

	requiresBearer := false
	allowsAPIKey := false
	requiredAPIKeyScopesSet := map[string]struct{}{}
	for _, secEntry := range securityEntries {
		secMap, secOK := secEntry.(map[string]any)
		if !secOK {
			continue
		}
		for schemeName, rawScopes := range secMap {
			switch schemeName {
			case "platformBearerAuth":
				requiresBearer = true
			case "productApiKeyAuth":
				allowsAPIKey = true
				scopes, _ := rawScopes.([]any)
				for _, rawScope := range scopes {
					scope, scopeOK := rawScope.(string)
					if scopeOK && scope != "" {
						requiredAPIKeyScopesSet[scope] = struct{}{}
					}
				}
			}
		}
	}

	requiredAPIKeyScopes := make([]string, 0, len(requiredAPIKeyScopesSet))
	for scope := range requiredAPIKeyScopesSet {
		requiredAPIKeyScopes = append(requiredAPIKeyScopes, scope)
	}
	sort.Strings(requiredAPIKeyScopes)

	return true, false, requiresBearer, allowsAPIKey, requiredAPIKeyScopes
}

func getRuntimePathParams(
	t *testing.T,
	route routeSecurityCase,
	fx *securityFixture,
) map[string]string {
	t.Helper()

	productID := fx.productContext.ProductID
	if route.OperationID == "deleteProduct" {
		productID = ids.MustNew("test")
	}

	return map[string]string{
		"product_id":    productID,
		"provider_type": "clerk",
	}
}

func replacePathParams(path string, params map[string]string) string {
	re := regexp.MustCompile(`\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(
		path,
		func(match string) string {
			name := strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
			if value, ok := params[name]; ok {
				return value
			}
			return ids.MustNew("test")
		},
	)
}

// extractParameters extracts required path and query parameters from an
// operation. A query parameter is resolved to a fake value satisfying its
// schema — an enum-constrained one in particular, since a value outside its
// enum is refused before the request ever reaches the auth check this suite
// exists to exercise.
func extractParameters(
	opMap map[string]any,
	pathMethods map[string]any,
	components, paramDefs map[string]any,
) (map[string]struct{}, map[string]string) {
	requiredPathParams := map[string]struct{}{}
	requiredQueryParams := map[string]string{}

	parseParams := func(raw any) {
		paramsArr, ok := raw.([]any)
		if !ok {
			return
		}
		for _, p := range paramsArr {
			param := resolveParameter(p, paramDefs)
			if param == nil {
				continue
			}
			if param["in"] == "path" && param["required"] == true {
				if name, nameOK := param["name"].(string); nameOK {
					requiredPathParams[name] = struct{}{}
				}
			}
			if param["in"] == "query" && param["required"] == true {
				if name, nameOK := param["name"].(string); nameOK {
					schema, _ := param["schema"].(map[string]any)
					requiredQueryParams[name] = fmt.Sprint(fakeValueForSchema(schema, components))
				}
			}
		}
	}

	parseParams(pathMethods["parameters"])
	parseParams(opMap["parameters"])

	return requiredPathParams, requiredQueryParams
}

func resolveParameter(p any, paramDefs map[string]any) map[string]any {
	refObj, ok := p.(map[string]any)
	if !ok {
		return nil
	}

	ref, hasRef := refObj["$ref"].(string)
	if hasRef && strings.HasPrefix(ref, "#/components/parameters/") {
		refName := strings.TrimPrefix(ref, "#/components/parameters/")
		if def, found := paramDefs[refName]; found {
			if refDefMap, refOK := def.(map[string]any); refOK {
				return refDefMap
			}
		}
		return nil
	}

	return refObj
}

func checkIfRequiresBody(opMap map[string]any) bool {
	rb, ok := opMap["requestBody"]
	if !ok {
		return false
	}
	rbMap, ok := rb.(map[string]any)
	if !ok {
		return false
	}
	required, ok := rbMap["required"].(bool)
	return ok && required
}

func buildURL(baseURL, path string, requiredQueryParams map[string]string) string {
	url := baseURL + path
	if len(requiredQueryParams) == 0 {
		return url
	}

	q := make([]string, 0, len(requiredQueryParams))
	for key, value := range requiredQueryParams {
		q = append(q, key+"="+neturl.QueryEscape(value))
	}
	sort.Strings(q)

	return url + "?" + strings.Join(q, "&")
}

func sendRequest(
	t *testing.T,
	route routeSecurityCase,
	components map[string]any,
	headers map[string]string,
	fx *securityFixture,
) *http.Response {
	t.Helper()

	urlPath := replacePathParams(route.Path, getRuntimePathParams(t, route, fx))
	url := buildURL(itshared.ServerURL, urlPath, route.RequiredQueryParams)

	var body io.Reader
	if route.RequiresBody {
		body = bytes.NewReader([]byte(generateRequestBody(route.OpMap, components)))
	}

	req, err := http.NewRequest(route.Method, url, body)
	require.NoError(t, err)
	if route.RequiresBody {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func generateRequestBody(opMap map[string]any, components map[string]any) string {
	rb, ok := opMap["requestBody"].(map[string]any)
	if !ok {
		return `{}`
	}
	content, ok := rb["content"].(map[string]any)
	if !ok {
		return `{}`
	}
	appJSON, ok := content["application/json"].(map[string]any)
	if !ok {
		return `{}`
	}
	schema, ok := appJSON["schema"].(map[string]any)
	if !ok {
		return `{}`
	}

	fake := generateFakeJSONFromSchema(schema, components)
	b, err := json.Marshal(fake)
	if err != nil {
		return `{}`
	}
	return string(b)
}

func generateFakeJSONFromSchema(schema, components map[string]any) any {
	if schema == nil {
		return map[string]any{}
	}

	if refSchema := resolveSchemaRef(schema, components); refSchema != nil {
		return generateFakeJSONFromSchema(refSchema, components)
	}

	typ, _ := schema["type"].(string)
	switch typ {
	case "object":
		obj := map[string]any{}
		props, _ := schema["properties"].(map[string]any)
		requiredFields := map[string]struct{}{}
		if requiredArr, ok := schema["required"].([]any); ok {
			for _, rawField := range requiredArr {
				field, fieldOK := rawField.(string)
				if fieldOK {
					requiredFields[field] = struct{}{}
				}
			}
		}
		for key, value := range props {
			if _, required := requiredFields[key]; !required {
				continue
			}
			propSchema, _ := value.(map[string]any)
			obj[key] = fakeValueForSchema(propSchema, components)
		}
		return obj
	case "array":
		items, _ := schema["items"].(map[string]any)
		return []any{fakeValueForSchema(items, components)}
	default:
		return map[string]any{}
	}
}

func resolveSchemaRef(schema, components map[string]any) map[string]any {
	ref, ok := schema["$ref"].(string)
	if !ok || !strings.HasPrefix(ref, "#/components/schemas/") {
		return nil
	}

	refName := strings.TrimPrefix(ref, "#/components/schemas/")
	allSchemas, ok := components["schemas"].(map[string]any)
	if !ok {
		return nil
	}
	refSchema, ok := allSchemas[refName].(map[string]any)
	if !ok {
		return nil
	}
	return refSchema
}

func fakeValueForSchema(schema, components map[string]any) any {
	if schema == nil {
		return nil
	}

	if ref, ok := schema["$ref"].(string); ok {
		if ref == "#/components/schemas/ksuid" {
			return ids.MustNew("test")
		}
		if resolved := resolveSchemaRef(schema, components); resolved != nil {
			return fakeValueForSchema(resolved, components)
		}
	}

	typ, _ := schema["type"].(string)
	switch typ {
	case "string":
		format, _ := schema["format"].(string)
		switch format {
		case "email":
			return "security-test@example.com"
		case "date-time":
			return "2024-01-01T00:00:00Z"
		default:
			if enumVals, ok := schema["enum"].([]any); ok && len(enumVals) > 0 {
				if enumStr, enumOK := enumVals[0].(string); enumOK {
					return enumStr
				}
			}
			if minLenRaw, ok := schema["minLength"].(int); ok && minLenRaw > 0 {
				return strings.Repeat("a", minLenRaw)
			}
			if minLenRaw, ok := schema["minLength"].(int64); ok && minLenRaw > 0 {
				return strings.Repeat("a", int(minLenRaw))
			}
			if minLenRaw, ok := schema["minLength"].(float64); ok && minLenRaw > 0 {
				return strings.Repeat("a", int(minLenRaw))
			}
			return "test"
		}
	case "integer":
		return int32(1)
	case "number":
		return 1
	case "boolean":
		return true
	case "array":
		items, _ := schema["items"].(map[string]any)
		return []any{fakeValueForSchema(items, components)}
	case "object":
		return generateFakeJSONFromSchema(schema, components)
	default:
		return "test"
	}
}

func validHeadersForRoute(
	t *testing.T,
	route routeSecurityCase,
	fx *securityFixture,
) map[string]string {
	t.Helper()

	if route.AllowsAPIKey {
		requiredScopes := route.RequiredAPIKeyScopes
		if len(requiredScopes) == 0 {
			requiredScopes = fx.allAPIKeyScopes
		}
		cred := fx.getOrCreateAPIKey(t, requiredScopes)
		return map[string]string{productAPIKeyHeader: cred.Value}
	}

	if route.RequiresBearer {
		return map[string]string{"Authorization": bearerPrefix + fx.bearerToken}
	}

	return map[string]string{}
}

func missingScopeHeadersForRoute(
	t *testing.T,
	route routeSecurityCase,
	fx *securityFixture,
) map[string]string {
	t.Helper()

	if route.AllowsAPIKey && len(route.RequiredAPIKeyScopes) > 0 {
		scopes := make([]string, 0, len(fx.allAPIKeyScopes))
		requiredSet := map[string]struct{}{}
		for _, scope := range route.RequiredAPIKeyScopes {
			requiredSet[scope] = struct{}{}
		}
		for _, scope := range fx.allAPIKeyScopes {
			if _, required := requiredSet[scope]; required {
				continue
			}
			scopes = append(scopes, scope)
		}
		cred := fx.getOrCreateAPIKey(t, scopes)
		return map[string]string{productAPIKeyHeader: cred.Value}
	}

	cred := fx.getOrCreateAPIKey(t, fx.allAPIKeyScopes)
	return map[string]string{productAPIKeyHeader: cred.Value}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	if resp == nil || resp.Body == nil {
		return ""
	}
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

func assertNoAuthError(t *testing.T, resp *http.Response, route routeSecurityCase, scenario string) {
	t.Helper()
	require.NotEqual(
		t,
		http.StatusUnauthorized,
		resp.StatusCode,
		"expected non-401 for %s (%s), got body: %s",
		route.OperationID,
		scenario,
		readBody(t, resp),
	)
	require.NotEqual(
		t,
		http.StatusForbidden,
		resp.StatusCode,
		"expected non-403 for %s (%s), got body: %s",
		route.OperationID,
		scenario,
		readBody(t, resp),
	)
}

func assertAuthRejected(t *testing.T, resp *http.Response, route routeSecurityCase, scenario string) {
	t.Helper()

	expectedStatus := http.StatusUnauthorized
	if route.AllowsAPIKey && len(route.RequiredAPIKeyScopes) > 0 {
		expectedStatus = http.StatusForbidden
	}

	require.Equalf(
		t,
		expectedStatus,
		resp.StatusCode,
		"expected %d for %s (%s), got %d with body: %s",
		expectedStatus,
		route.OperationID,
		scenario,
		resp.StatusCode,
		readBody(t, resp),
	)
}

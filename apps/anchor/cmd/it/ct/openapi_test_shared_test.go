package ct_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/nanostack-dev/shared/toolkit"
)

// sendRequest sends an HTTP request for an operation, with optional headers.
func sendRequest(
	op extractedOp, components map[string]interface{}, headers map[string]string,
) (*http.Response, error) {
	switch op.Verb {
	case "get":
		return sendGetRequest(op.URL, headers)
	case "delete":
		return sendDeleteRequest(op.URL, headers)
	case "post", "put", "patch":
		return sendRequestWithBody(op, components, headers)
	default:
		return nil, nil //
	}
}

// sendGetRequest sends a GET request with headers.
func sendGetRequest(url string, headers map[string]string) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	addHeaders(req, headers)
	return http.DefaultClient.Do(req)
}

// sendDeleteRequest sends a DELETE request with headers.
func sendDeleteRequest(url string, headers map[string]string) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	addHeaders(req, headers)
	return http.DefaultClient.Do(req)
}

// sendRequestWithBody sends a request with a body (POST, PUT, PATCH).
func sendRequestWithBody(
	op extractedOp, components map[string]interface{}, headers map[string]string,
) (*http.Response, error) {
	bodyStr := generateRequestBody(op, components)
	var bodyReader io.Reader
	if bodyStr != "" {
		bodyReader = strings.NewReader(bodyStr)
	}

	method := strings.ToUpper(op.Verb)
	req, _ := http.NewRequest(method, op.URL, bodyReader)

	if op.RequiresBody {
		req.Header.Set("Content-Type", "application/json")
	}
	addHeaders(req, headers)
	return http.DefaultClient.Do(req)
}

// generateRequestBody generates the request body for operations that require it.
func generateRequestBody(op extractedOp, components map[string]interface{}) string {
	if !op.RequiresBody {
		return ""
	}

	rb, ok := op.OpMap["requestBody"].(map[string]interface{})
	if !ok {
		return `{"test":"test"}`
	}

	content, ok := rb["content"].(map[string]interface{})
	if !ok {
		return `{"test":"test"}`
	}

	appjson, ok := content["application/json"].(map[string]interface{})
	if !ok {
		return `{"test":"test"}`
	}

	schema, ok := appjson["schema"].(map[string]interface{})
	if !ok {
		return `{"test":"test"}`
	}

	fake := generateFakeJSONFromSchema(schema, components)
	b, _ := json.Marshal(fake)
	return string(b)
}

// addHeaders adds headers to an HTTP request.
func addHeaders(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

// OpenAPI represents the OpenAPI spec structure for paths.
type OpenAPI struct {
	Paths map[string]map[string]interface{} `yaml:"paths"`
}

// Operation represents an OpenAPI operation.
type Operation struct {
	Security  []map[string]interface{} `yaml:"security"`
	Responses map[string]interface{}   `yaml:"responses"`
}

// extractedOp holds extracted operation details for testing.
type extractedOp struct {
	Verb                string
	OpKey               string
	URL                 string
	OpMap               map[string]interface{}
	RequiresBody        bool
	RequiredPathParams  map[string]struct{}
	RequiredQueryParams []string
}

// parseOpenAPISpec parses the OpenAPI YAML and extracts components and parameter definitions.
func parseOpenAPISpec(t *testing.T) (OpenAPI, map[string]interface{}, map[string]interface{}) {
	data, err := os.ReadFile("../../http/openapi.yaml")
	if err != nil {
		t.Fatalf("failed to read openapi.yaml: %v", err)
	}
	var spec OpenAPI
	if specErr := yaml.Unmarshal(data, &spec); specErr != nil {
		t.Fatalf("failed to parse openapi.yaml: %v", specErr)
	}
	var specRaw map[string]interface{}
	if rawErr := yaml.Unmarshal(data, &specRaw); rawErr != nil {
		t.Fatalf("failed to parse openapi.yaml (raw): %v", rawErr)
	}
	components := map[string]interface{}{}
	if c, ok := specRaw["components"].(map[string]interface{}); ok {
		components = c
	}
	paramDefs := map[string]interface{}{}
	if p, ok := components["parameters"].(map[string]interface{}); ok {
		paramDefs = p
	}
	return spec, components, paramDefs
}

// extractOperations extracts protected operations from the OpenAPI spec.
func extractOperations(
	spec OpenAPI, paramDefs map[string]interface{}, noAuth map[string]struct{},
	baseURL string,
) []extractedOp {
	var ops []extractedOp
	for origPath, methods := range spec.Paths {
		pathOps := extractOperationsFromPath(
			origPath, methods, paramDefs, noAuth, baseURL,
		)
		ops = append(ops, pathOps...)
	}
	return ops
}

// extractOperationsFromPath extracts operations from a single path.
func extractOperationsFromPath(
	origPath string, methods map[string]interface{}, paramDefs map[string]interface{},
	noAuth map[string]struct{}, baseURL string,
) []extractedOp {
	var ops []extractedOp
	for method, rawOp := range methods {
		op := extractSingleOperation(
			origPath, method, rawOp, paramDefs, noAuth, baseURL,
		)
		if op != nil {
			ops = append(ops, *op)
		}
	}
	return ops
}

// extractSingleOperation extracts a single operation from a method.
func extractSingleOperation(
	origPath, method string, rawOp interface{}, paramDefs map[string]interface{},
	noAuth map[string]struct{}, baseURL string,
) *extractedOp {
	opMap, ok := rawOp.(map[string]interface{})
	if !ok {
		return nil
	}

	var op Operation
	opBytes, err := yaml.Marshal(opMap)
	if err != nil {
		return nil
	}
	if unmarshalErr := yaml.Unmarshal(opBytes, &op); unmarshalErr != nil {
		return nil
	}

	opKey := method + " " + strings.ReplaceAll(strings.ReplaceAll(origPath, "{", ""), "}", "")
	if _, skip := noAuth[opKey]; skip {
		return nil
	}
	if len(op.Security) == 0 {
		return nil
	}

	requiresBody := checkIfRequiresBody(opMap)
	requiredPathParams, requiredQueryParams := extractParameters(opMap, paramDefs)

	path := regexp.MustCompile(`\{[^}]+}`).ReplaceAllString(origPath, toolkit.NewID("test"))
	url := buildURL(baseURL, path, requiredQueryParams)

	return &extractedOp{
		Verb:                method,
		OpKey:               opKey,
		URL:                 url,
		OpMap:               opMap,
		RequiresBody:        requiresBody,
		RequiredPathParams:  requiredPathParams,
		RequiredQueryParams: requiredQueryParams,
	}
}

// checkIfRequiresBody checks if the operation requires a request body.
func checkIfRequiresBody(opMap map[string]interface{}) bool {
	rb, ok := opMap["requestBody"]
	if !ok {
		return false
	}

	rbMap, ok := rb.(map[string]interface{})
	if !ok {
		return false
	}

	req, ok := rbMap["required"].(bool)
	return ok && req
}

// extractParameters extracts required path and query parameters from operation.
func extractParameters(opMap, paramDefs map[string]interface{}) (map[string]struct{}, []string) {
	requiredPathParams := map[string]struct{}{}
	requiredQueryParams := []string{}

	paramsRaw, ok := opMap["parameters"]
	if !ok {
		return requiredPathParams, requiredQueryParams
	}

	paramsArr, ok := paramsRaw.([]interface{})
	if !ok {
		return requiredPathParams, requiredQueryParams
	}

	for _, p := range paramsArr {
		param := resolveParameter(p, paramDefs)
		if param == nil {
			continue
		}

		if param["in"] == "path" && param["required"] == true {
			if pathName, pathOk := param["name"].(string); pathOk {
				requiredPathParams[pathName] = struct{}{}
			}
		}

		if param["in"] == "query" && param["required"] == true {
			if queryName, queryOk := param["name"].(string); queryOk {
				requiredQueryParams = append(requiredQueryParams, queryName)
			}
		}
	}

	return requiredPathParams, requiredQueryParams
}

// resolveParameter resolves a parameter definition, handling $ref if present.
func resolveParameter(p interface{}, paramDefs map[string]interface{}) map[string]interface{} {
	refObj, ok := p.(map[string]interface{})
	if !ok {
		return nil
	}

	ref, hasRef := refObj["$ref"].(string)
	if hasRef && strings.HasPrefix(ref, "#/components/parameters/") {
		refName := strings.TrimPrefix(ref, "#/components/parameters/")
		if def, found := paramDefs[refName]; found {
			if refDefMap, refOk := def.(map[string]interface{}); refOk {
				return refDefMap
			}
		}
		return nil
	}

	return refObj
}

// buildURL builds the final URL with query parameters if needed.
func buildURL(baseURL, path string, requiredQueryParams []string) string {
	url := baseURL + path
	if len(requiredQueryParams) == 0 {
		return url
	}

	q := make([]string, 0, len(requiredQueryParams))
	for _, k := range requiredQueryParams {
		q = append(q, k+"=test")
	}

	if strings.Contains(url, "?") {
		url += "&" + strings.Join(q, "&")
	} else {
		url += "?" + strings.Join(q, "&")
	}

	return url
}

// generateFakeJSONFromSchema generates a fake JSON object from an OpenAPI schema.
func generateFakeJSONFromSchema(
	schema map[string]interface{}, components map[string]interface{},
) interface{} {
	if schema == nil {
		return map[string]interface{}{}
	}

	// Handle $ref
	if refSchema := resolveSchemaRef(schema, components); refSchema != nil {
		return generateFakeJSONFromSchema(refSchema, components)
	}

	typ, _ := schema["type"].(string)
	switch typ {
	case "object":
		return generateFakeObjectFromSchema(schema, components)
	case "array":
		return generateFakeArrayFromSchema(schema, components)
	default:
		return map[string]interface{}{}
	}
}

// resolveSchemaRef resolves a schema reference if present.
func resolveSchemaRef(schema, components map[string]interface{}) map[string]interface{} {
	ref, ok := schema["$ref"].(string)
	if !ok || !strings.HasPrefix(ref, "#/components/schemas/") {
		return nil
	}

	refName := strings.TrimPrefix(ref, "#/components/schemas/")
	compSchemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		return nil
	}

	realSchema, ok := compSchemas[refName].(map[string]interface{})
	if !ok {
		return nil
	}

	return realSchema
}

// generateFakeObjectFromSchema generates a fake object from an object schema.
func generateFakeObjectFromSchema(schema, components map[string]interface{}) map[string]interface{} {
	obj := map[string]interface{}{}
	props, _ := schema["properties"].(map[string]interface{})
	required := extractRequiredFields(schema)

	for k, v := range props {
		propSchema, _ := v.(map[string]interface{})
		// Only fill required fields
		if _, isReq := required[k]; isReq {
			obj[k] = fakeValueForSchema(propSchema, components)
		}
	}

	return obj
}

// extractRequiredFields extracts required fields from a schema.
func extractRequiredFields(schema map[string]interface{}) map[string]struct{} {
	required := map[string]struct{}{}
	reqArr, ok := schema["required"].([]interface{})
	if !ok {
		return required
	}

	for _, r := range reqArr {
		if fieldName, fieldOk := r.(string); fieldOk {
			required[fieldName] = struct{}{}
		}
	}

	return required
}

// generateFakeArrayFromSchema generates a fake array from an array schema.
func generateFakeArrayFromSchema(schema, components map[string]interface{}) []interface{} {
	items, _ := schema["items"].(map[string]interface{})
	return []interface{}{fakeValueForSchema(items, components)}
}

// fakeValueForSchema generates a fake value for a given schema property.
func fakeValueForSchema(
	schema map[string]interface{}, components map[string]interface{},
) interface{} {
	if schema == nil {
		return nil
	}

	// Handle $ref
	if refValue := resolveFakeValueRef(schema, components); refValue != nil {
		return refValue
	}

	typ, _ := schema["type"].(string)
	return generateFakeValueByType(typ, schema, components)
}

// resolveFakeValueRef resolves a reference in fake value generation.
func resolveFakeValueRef(schema, components map[string]interface{}) interface{} {
	ref, ok := schema["$ref"].(string)
	if !ok {
		return nil
	}

	if ref == "#/components/schemas/ksuid" {
		return toolkit.NewID("test")
	}

	if !strings.HasPrefix(ref, "#/components/schemas/") {
		return nil
	}

	refName := strings.TrimPrefix(ref, "#/components/schemas/")
	compSchemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		return nil
	}

	realSchema, ok := compSchemas[refName].(map[string]interface{})
	if !ok {
		return nil
	}

	return fakeValueForSchema(realSchema, components)
}

// generateFakeValueByType generates a fake value based on the schema type.
func generateFakeValueByType(typ string, schema, components map[string]interface{}) interface{} {
	switch typ {
	case "string":
		return generateFakeStringValue(schema)
	case "integer", "number":
		return 1
	case "boolean":
		return true
	case "array":
		items, _ := schema["items"].(map[string]interface{})
		return []interface{}{fakeValueForSchema(items, components)}
	case "object":
		return generateFakeJSONFromSchema(schema, components)
	default:
		return "test"
	}
}

// generateFakeStringValue generates a fake string value based on format.
func generateFakeStringValue(schema map[string]interface{}) string {
	format, _ := schema["format"].(string)
	switch format {
	case "email":
		return "test@example.com"
	case "date-time":
		return "2024-01-01T00:00:00Z"
	default:
		return "test"
	}
}

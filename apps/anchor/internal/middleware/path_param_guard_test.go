package middleware_test

import (
	"regexp"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var maintainedPathParams = map[string]struct{}{
	"api_key_id":              {},
	"email_template_id":       {},
	"integration_instance_id": {},
	"invitation_id":           {},
	"license_template_id":     {},
	"organization_id":         {},
	"permission_id":           {},
	"permission_name":         {},
	"platform_user_id":        {},
	"product_id":              {},
	"product_user_id":         {},
	"provider_type":           {},
	"role_id":                 {},
	"workspace_id":            {},
}

func TestOpenAPIPathVariablesStayOnMaintainedList(t *testing.T) {
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromFile("../../cmd/http/openapi.yaml")
	require.NoError(t, err)

	pathVariablePattern := regexp.MustCompile(`\{([^}]+)\}`)
	seen := make(map[string]struct{})

	for path := range spec.Paths.Map() {
		matches := pathVariablePattern.FindAllStringSubmatch(path, -1)
		for _, match := range matches {
			require.Len(t, match, 2)
			name := match[1]
			seen[name] = struct{}{}
			_, ok := maintainedPathParams[name]
			assert.Truef(t, ok, "path variable %q from path %q is missing from maintainedPathParams", name, path)
		}
	}

	for name := range maintainedPathParams {
		_, ok := seen[name]
		assert.Truef(t, ok, "maintained path variable %q is no longer present in the OpenAPI spec", name)
	}
}

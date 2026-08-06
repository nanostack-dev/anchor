package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLicenseSchemaValidation walks every way a declaration can be malformed.
// Each case asserts the structured shape a caller renders from — the offending
// license field and the rule it violated — not the message prose.
//
// This is the authoring-time half of "rules constrain decisions, not
// observations": a nonsensical declaration is refused when the schema is
// written, rather than the first time a value is checked against it.
func TestLicenseSchemaValidation(t *testing.T) {
	cases := []struct {
		name  string
		field ct.LicenseFieldDeclaration
		code  string
		rule  string
	}{
		{
			name:  "unknown field type",
			field: ct.LicenseFieldDeclaration{Name: "flows", Type: ct.LicenseFieldType("counter")},
			code:  "LICENSE_FIELD_RULE_INVALID",
			rule:  "type",
		},
		{
			name: "min above max",
			field: ct.LicenseFieldDeclaration{
				Name:  "flows",
				Type:  ct.LicenseFieldTypeLimit,
				Rules: limitRules(100, 10),
			},
			code: "LICENSE_FIELD_RULE_INVALID",
			rule: "min",
		},
		{
			name: "negative minimum on a limit",
			field: ct.LicenseFieldDeclaration{
				Name:  "flows",
				Type:  ct.LicenseFieldTypeLimit,
				Rules: &ct.LicenseFieldRules{Min: new(-1.0)},
			},
			code: "LICENSE_FIELD_RULE_INVALID",
			rule: "min",
		},
		{
			name: "regular expression that does not compile",
			field: ct.LicenseFieldDeclaration{
				Name:  "region",
				Type:  ct.LicenseFieldTypeString,
				Rules: &ct.LicenseFieldRules{Pattern: new("[unterminated")},
			},
			code: "LICENSE_FIELD_RULE_INVALID",
			rule: "pattern",
		},
		{
			name: "enum with no allowed values",
			field: ct.LicenseFieldDeclaration{
				Name:  "support_tier",
				Type:  ct.LicenseFieldTypeEnum,
				Rules: &ct.LicenseFieldRules{Values: enumValues()},
			},
			code: "LICENSE_FIELD_RULE_INVALID",
			rule: "values",
		},
		{
			name: "enum with a duplicate allowed value",
			field: ct.LicenseFieldDeclaration{
				Name:  "support_tier",
				Type:  ct.LicenseFieldTypeEnum,
				Rules: &ct.LicenseFieldRules{Values: enumValues("basic", "basic")},
			},
			code: "LICENSE_FIELD_RULE_INVALID",
			rule: "values",
		},
		{
			name: "numeric bound on a boolean field",
			field: ct.LicenseFieldDeclaration{
				Name:  "sso",
				Type:  ct.LicenseFieldTypeBoolean,
				Rules: &ct.LicenseFieldRules{Max: new(1.0)},
			},
			code: "LICENSE_FIELD_RULE_INVALID",
			rule: "max",
		},
		{
			name: "pattern on a numeric field",
			field: ct.LicenseFieldDeclaration{
				Name:  "flows",
				Type:  ct.LicenseFieldTypeLimit,
				Rules: &ct.LicenseFieldRules{Pattern: new("^[0-9]+$")},
			},
			code: "LICENSE_FIELD_RULE_INVALID",
			rule: "pattern",
		},
		{
			name: "allowed values on a string field",
			field: ct.LicenseFieldDeclaration{
				Name:  "region",
				Type:  ct.LicenseFieldTypeString,
				Rules: &ct.LicenseFieldRules{Values: enumValues("ca-central")},
			},
			code: "LICENSE_FIELD_RULE_INVALID",
			rule: "values",
		},
		{
			name: "length bounds that cross",
			field: ct.LicenseFieldDeclaration{
				Name:  "region",
				Type:  ct.LicenseFieldTypeString,
				Rules: &ct.LicenseFieldRules{MinLength: new(20), MaxLength: new(5)},
			},
			code: "LICENSE_FIELD_RULE_INVALID",
			rule: "min_length",
		},
	}

	for _, c := range cases {
		t.Run("create rejects "+c.name, func(t *testing.T) {
			tc := newTestCtx(t)

			resp, err := tc.product.OwnerAuthenticatedClient().CreateLicenseSchemaWithResponse(
				context.Background(),
				tc.product.ProductID,
				ct.CreateLicenseSchemaJSONRequestBody{Fields: []ct.LicenseFieldDeclaration{c.field}},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
			require.NotNil(t, resp.JSON400)
			assertFieldError(t, resp.JSON400.Errors, c.code, c.field.Name, c.rule)

			// The rejected write left nothing behind.
			read, readErr := tc.product.OwnerAuthenticatedClient().GetLicenseSchemaWithResponse(
				context.Background(), tc.product.ProductID,
			)
			require.NoError(t, readErr)
			assert.Equal(t, http.StatusNotFound, read.StatusCode())
		})

		t.Run("update rejects "+c.name, func(t *testing.T) {
			tc := newTestCtx(t)
			client := seedSchema(t, tc)

			resp, err := client.UpdateLicenseSchemaWithResponse(
				context.Background(),
				tc.product.ProductID,
				ct.UpdateLicenseSchemaJSONRequestBody{
					Fields: &[]ct.LicenseFieldDeclaration{c.field},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
			require.NotNil(t, resp.JSON400)
			assertFieldError(t, resp.JSON400.Errors, c.code, c.field.Name, c.rule)
		})
	}

	t.Run("rejects a field name declared twice", func(t *testing.T) {
		tc := newTestCtx(t)
		name := uniqueFieldName()

		resp, err := tc.product.OwnerAuthenticatedClient().CreateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseSchemaJSONRequestBody{
				Fields: []ct.LicenseFieldDeclaration{
					{Name: name, Type: ct.LicenseFieldTypeLimit},
					{Name: name, Type: ct.LicenseFieldTypeBoolean},
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_NAME_DUPLICATE", name, "")
	})

	t.Run("rejects a field with no name", func(t *testing.T) {
		tc := newTestCtx(t)

		resp, err := tc.product.OwnerAuthenticatedClient().CreateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseSchemaJSONRequestBody{
				Fields: []ct.LicenseFieldDeclaration{{Name: "", Type: ct.LicenseFieldTypeLimit}},
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	})
}

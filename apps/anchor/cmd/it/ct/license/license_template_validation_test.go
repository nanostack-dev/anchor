package license_ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLicenseTemplateValidation pins every way a template write is refused.
// A template is validated against its product's license schema on every write,
// so this is where "I cannot ship a template that violates my own declaration"
// is actually proved.
//
// Each case starts from a complete, valid set and breaks exactly one thing.
// That is deliberate: every declared license field is mandatory, so a set
// assembled ad hoc would trip the missing-field check first and the case would
// pass without ever reaching the rule it names.
func TestLicenseTemplateValidation(t *testing.T) {
	t.Run("rejects a template omitting a declared field", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), templateValuesExcept("sso"))
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_MISSING", "sso", "")
	})

	t.Run("rejects a value above its field's maximum", func(t *testing.T) {
		tc := newTemplateCtx(t)

		// The declared maximum is 100000.
		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), templateValuesWith("flows", 100001))
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "flows", "max")
	})

	t.Run("rejects a value below its field's minimum", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), templateValuesWith("flows", -1))
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "flows", "min")
	})

	t.Run("rejects a value outside an enum's allowed list", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(
			t, tc, uniqueTemplateName(), templateValuesWith("support_tier", "platinum"),
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "support_tier", "values")
	})

	t.Run("rejects a string that does not match its field's pattern", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(
			t, tc, uniqueTemplateName(), templateValuesWith("region", "CA_CENTRAL"),
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "region", "pattern")
	})

	t.Run("rejects a string that only partially matches its field's pattern", func(t *testing.T) {
		// The "region" fixture's pattern is anchored (^...$), so a value that
		// merely contains no valid substring at all would fail whether or not
		// the match is anchored, and would not isolate the bug. This declares
		// its own unanchored pattern instead, matching what a schema author is
		// free to write, and a value that contains a matching substring
		// without being wholly one: "yoW" against "[a-z]+".
		tc := newTestCtx(t)
		declareSchema(t, tc, []ct.LicenseFieldDeclaration{
			{
				Name:  "code",
				Type:  ct.LicenseFieldTypeSTRING,
				Rules: &ct.LicenseFieldRules{Pattern: new("[a-z]+")},
			},
		})

		resp, err := createTemplateRaw(
			t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{"code": "yoW"},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "code", "pattern")
	})

	t.Run("rejects a value of the wrong type for its field", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(
			t, tc, uniqueTemplateName(), templateValuesWith("flows", "unlimited"),
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "flows", "type")
	})

	t.Run("rejects a field absent from the schema", func(t *testing.T) {
		tc := newTemplateCtx(t)

		// An undeclared key is refused even though every declared field is set,
		// so a typo cannot be stored as a value nothing will ever read.
		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), templateValuesWith("seat_cap", 12))
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_UNKNOWN", "seat_cap", "")
	})

	t.Run("rejects a second template with the same name", func(t *testing.T) {
		tc := newTemplateCtx(t)
		createTemplate(t, tc, "Pro", validTemplateValues())

		resp, err := createTemplateRaw(t, tc, "Pro", validTemplateValues())
		require.NoError(t, err)
		// Archiving or renaming the template holding the name is the later
		// request that frees it, the same shape as a uniqueness rule — a 409,
		// not a 400. The generated client has no typed 409 getter for this
		// route yet, so the body is decoded by hand.
		require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
		var errResp ct.ApiErrorResponse
		require.NoError(t, json.Unmarshal(resp.Body, &errResp))
		assertFieldError(t, errResp.Errors, "LICENSE_TEMPLATE_NAME_EXISTS", "name", "")
	})

	t.Run("refuses a template for a product with no license schema", func(t *testing.T) {
		// Deliberately not newTemplateCtx: a template is defined as a set of
		// values satisfying a schema, so without one there is nothing to satisfy.
		tc := newTestCtx(t)

		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{})
		require.NoError(t, err)
		// This call never names the schema itself, on the path or in the
		// body, so a missing schema is not the 404 the schema route answers.
		// It is a 409: declaring a schema is the later request that lets this
		// exact call succeed.
		assert.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	})

	t.Run("accepts a template setting nothing when the schema declares nothing", func(t *testing.T) {
		tc := newTestCtx(t)
		declareSchema(t, tc, []ct.LicenseFieldDeclaration{})

		template := createTemplate(t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{})
		assert.Empty(t, template.Values)
	})
}

// TestLicenseTemplateUpdateValidation proves the schema check is not a
// create-time courtesy. An edit that would leave a template violating the
// declaration is refused the same way the original write would have been.
func TestLicenseTemplateUpdateValidation(t *testing.T) {
	t.Run("rejects an edit that drops a declared field", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		// Values are replaced wholesale, so an omitted field is a removal — and a
		// removal is exactly what a mandatory field forbids.
		replacement := templateValuesExcept("sso")
		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			created.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Values: &replacement},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_MISSING", "sso", "")
	})

	t.Run("rejects an edit whose value violates a rule", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		replacement := templateValuesWith("flows", 250000)
		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			created.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Values: &replacement},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "flows", "max")
	})

	t.Run("rejects an edit introducing an undeclared field", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		replacement := templateValuesWith("seat_cap", 12)
		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			created.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Values: &replacement},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_UNKNOWN", "seat_cap", "")
	})

	t.Run("rejects a rename onto a name already taken", func(t *testing.T) {
		tc := newTemplateCtx(t)
		createTemplate(t, tc, "Pro", validTemplateValues())
		free := createTemplate(t, tc, "Free", templateValuesWith("flows", 10))

		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			free.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Name: new("Pro")},
		)
		require.NoError(t, err)
		// Same as create: a 409, not a 400. The generated client has no typed
		// 409 getter for this route yet, so the body is decoded by hand.
		require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
		var errResp ct.ApiErrorResponse
		require.NoError(t, json.Unmarshal(resp.Body, &errResp))
		assertFieldError(t, errResp.Errors, "LICENSE_TEMPLATE_NAME_EXISTS", "name", "")
	})

	t.Run("accepts renaming a template to the name it already has", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, "Pro", validTemplateValues())

		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			created.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Name: new("Pro")},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	})
}

// createTemplateRaw is the write without the success assertion, for the tests
// whose subject is the refusal.
func createTemplateRaw(
	t *testing.T, tc testCtx, name string, values ct.LicenseTemplateValues,
) (*ct.CreateLicenseTemplateResponse, error) {
	t.Helper()
	return tc.product.OwnerAuthenticatedClient().CreateLicenseTemplateWithResponse(
		context.Background(),
		tc.product.ProductID,
		ct.CreateLicenseTemplateJSONRequestBody{Name: name, Values: values},
	)
}

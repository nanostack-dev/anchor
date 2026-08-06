package license_ct_test

import (
	"context"
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
func TestLicenseTemplateValidation(t *testing.T) {
	t.Run("rejects a template omitting a required field", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{
			"flows": 500,
			// sso is required and absent.
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_REQUIRED", "sso", "")
	})

	t.Run("rejects a value above its field's maximum", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{
			"flows": 100001, // the declared maximum is 100000
			"sso":   true,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "flows", "max")
	})

	t.Run("rejects a value below its field's minimum", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{
			"flows": -1,
			"sso":   true,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "flows", "min")
	})

	t.Run("rejects a value outside an enum's allowed list", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{
			"flows":        500,
			"sso":          true,
			"support_tier": "platinum",
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "support_tier", "values")
	})

	t.Run("rejects a string that does not match its field's pattern", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{
			"flows":  500,
			"sso":    true,
			"region": "CA_CENTRAL",
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "region", "pattern")
	})

	t.Run("rejects a value of the wrong type for its field", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{
			"flows": "unlimited",
			"sso":   true,
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "flows", "type")
	})

	t.Run("rejects a field absent from the schema", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{
			"flows":    500,
			"sso":      true,
			"seat_cap": 12, // never declared
		})
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
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_TEMPLATE_NAME_EXISTS", "name", "")
	})

	t.Run("refuses a template for a product with no license schema", func(t *testing.T) {
		// Deliberately not newTemplateCtx: a template is defined as a set of
		// values satisfying a schema, so without one there is nothing to satisfy.
		tc := newTestCtx(t)

		resp, err := createTemplateRaw(t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{})
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("accepts a template setting nothing when the schema requires nothing", func(t *testing.T) {
		tc := newTestCtx(t)
		declareSchema(t, tc, []ct.LicenseFieldDeclaration{
			{Name: "sso", Type: ct.LicenseFieldTypeBOOLEAN},
		})

		template := createTemplate(t, tc, uniqueTemplateName(), ct.LicenseTemplateValues{})
		assert.Empty(t, template.Values)
	})
}

// TestLicenseTemplateUpdateValidation proves the schema check is not a
// create-time courtesy. An edit that would leave a template violating the
// declaration is refused the same way the original write would have been.
func TestLicenseTemplateUpdateValidation(t *testing.T) {
	t.Run("rejects an edit that drops a required field", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		replacement := ct.LicenseTemplateValues{"flows": 500}
		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			created.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Values: &replacement},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_REQUIRED", "sso", "")
	})

	t.Run("rejects an edit whose value violates a rule", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		replacement := ct.LicenseTemplateValues{"flows": 250000, "sso": true}
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

		replacement := ct.LicenseTemplateValues{"flows": 500, "sso": true, "seat_cap": 12}
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
		free := createTemplate(t, tc, "Free", ct.LicenseTemplateValues{"flows": 10, "sso": false})

		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			free.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Name: new("Pro")},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_TEMPLATE_NAME_EXISTS", "name", "")
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

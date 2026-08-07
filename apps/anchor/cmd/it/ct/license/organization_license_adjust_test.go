package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adjust sends an adjustment and returns the raw response, so each test can
// assert on the status it expects rather than on one the helper decided.
func adjust(
	t *testing.T, lc licenseCtx, values ct.LicenseTemplateValues,
) *ct.AdjustOrganizationLicenseResponse {
	t.Helper()
	resp, err := lc.product.OwnerAuthenticatedClient().AdjustOrganizationLicenseWithResponse(
		context.Background(),
		lc.product.ProductID,
		lc.organizationID,
		ct.AdjustOrganizationLicenseJSONRequestBody{Values: values},
	)
	require.NoError(t, err)
	return resp
}

func TestAdjustOrganizationLicense(t *testing.T) {
	t.Run("adjusts one license field and leaves the rest alone", func(t *testing.T) {
		lc, _ := licensedOrganization(t)

		resp := adjust(t, lc, ct.LicenseTemplateValues{"flows": 800})
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		// A bespoke arrangement for one customer, expressed as an edit to that
		// customer's own record rather than as a new tier.
		assert.InDelta(t, 800.0, resp.JSON200.Values["flows"], 0)
		assert.Equal(t, true, resp.JSON200.Values["sso"])
		assert.Equal(t, "priority", resp.JSON200.Values["support_tier"])
		assert.Equal(t, "ca-central", resp.JSON200.Values["region"])
	})

	t.Run("adjusts several license fields at once", func(t *testing.T) {
		lc, _ := licensedOrganization(t)

		resp := adjust(t, lc, ct.LicenseTemplateValues{"flows": 900, "sso": false})
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		assert.InDelta(t, 900.0, resp.JSON200.Values["flows"], 0)
		assert.Equal(t, false, resp.JSON200.Values["sso"])
		assert.Equal(t, "priority", resp.JSON200.Values["support_tier"])
	})

	t.Run("an omitted license field is left alone rather than unset", func(t *testing.T) {
		lc, _ := licensedOrganization(t)

		// The opposite of a template write, where an omitted license field is a
		// removal. A license is adjusted one field at a time, so a request that
		// names one field must not silently revert the others.
		resp := adjust(t, lc, ct.LicenseTemplateValues{"flows": 800})
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)
		assert.Len(t, resp.JSON200.Values, len(templateSchemaFields()))
	})

	t.Run("leaves the provenance alone", func(t *testing.T) {
		lc, template := licensedOrganization(t)
		before := instantiatedAt(t, lc)

		resp := adjust(t, lc, ct.LicenseTemplateValues{"flows": 800})
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		// "Which template was this organization sold, and when" is a historical
		// fact. Raising their limit afterwards does not change what they bought.
		assert.Equal(t, template.Id, resp.JSON200.TemplateId)
		assert.Equal(t, before, resp.JSON200.InstantiatedAt)
	})

	t.Run("an empty adjustment changes nothing", func(t *testing.T) {
		lc, _ := licensedOrganization(t)

		resp := adjust(t, lc, ct.LicenseTemplateValues{})
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)
		assert.InDelta(t, 500.0, resp.JSON200.Values["flows"], 0)
	})

	t.Run("refuses a value outside its license field's rules", func(t *testing.T) {
		lc, _ := licensedOrganization(t)

		// Validated against the schema exactly as a template write is, and by the
		// same code — an adjustment is not a back door around the declaration.
		resp := adjust(t, lc, ct.LicenseTemplateValues{"flows": 100001})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "flows", "max")
	})

	t.Run("refuses a value of the wrong type", func(t *testing.T) {
		lc, _ := licensedOrganization(t)

		resp := adjust(t, lc, ct.LicenseTemplateValues{"sso": "yes"})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "sso", "type")
	})

	t.Run("refuses a license field the schema does not declare", func(t *testing.T) {
		lc, _ := licensedOrganization(t)

		resp := adjust(t, lc, ct.LicenseTemplateValues{"seats": 12})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_UNKNOWN", "seats", "")
	})

	t.Run("404 when the organization has no license", func(t *testing.T) {
		lc := newOrganizationLicenseCtx(t)

		resp := adjust(t, lc, ct.LicenseTemplateValues{"flows": 800})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("does not adjust another product's license", func(t *testing.T) {
		first, _ := licensedOrganization(t)
		second := newOrganizationLicenseCtx(t)

		resp, err := second.product.OwnerAuthenticatedClient().AdjustOrganizationLicenseWithResponse(
			context.Background(),
			second.product.ProductID,
			first.organizationID,
			ct.AdjustOrganizationLicenseJSONRequestBody{
				Values: ct.LicenseTemplateValues{"flows": 800},
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

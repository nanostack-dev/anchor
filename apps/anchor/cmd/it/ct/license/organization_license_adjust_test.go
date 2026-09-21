package license_ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdjustOrganizationLicense(t *testing.T) {
	t.Run("adjusts one license field and leaves the rest alone", func(t *testing.T) {
		w := newLicensedWorld(t)

		adjusted := w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})

		// A bespoke arrangement for one customer, expressed as an edit to that
		// customer's own record rather than as a new tier.
		assertValues(t, adjusted.Values, ct.LicenseTemplateValues{
			"flows": 800, "sso": true, "support_tier": "priority", "region": "ca-central",
		})
	})

	t.Run("adjusts several license fields at once", func(t *testing.T) {
		w := newLicensedWorld(t)

		adjusted := w.License().Adjust(ct.LicenseTemplateValues{"flows": 900, "sso": false})

		assertValues(t, adjusted.Values, ct.LicenseTemplateValues{
			"flows": 900, "sso": false, "support_tier": "priority", "region": "ca-central",
		})
	})

	t.Run("an omitted license field is left alone rather than unset", func(t *testing.T) {
		w := newLicensedWorld(t)

		// The opposite of a template write, where an omitted license field is a
		// removal. A license is adjusted one field at a time, so a request that
		// names one field must not silently revert the others.
		adjusted := w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})
		assert.Len(t, adjusted.Values, len(templateSchemaFields()))
	})

	t.Run("leaves the provenance alone", func(t *testing.T) {
		w := newLicensedWorld(t)
		before := w.License().Get()

		adjusted := w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})

		// "Which template was this organization sold, and when" is a historical
		// fact. Raising their limit afterwards does not change what they bought.
		assert.Equal(t, w.TemplateID(), adjusted.TemplateId)
		assert.Equal(t, before.InstantiatedAt, adjusted.InstantiatedAt)
	})

	t.Run("an empty adjustment changes nothing", func(t *testing.T) {
		w := newLicensedWorld(t)

		adjusted := w.License().Adjust(ct.LicenseTemplateValues{})

		assertValues(t, adjusted.Values, validTemplateValues())
	})

	t.Run("refuses a value outside its license field's rules", func(t *testing.T) {
		w := newLicensedWorld(t)

		// Validated against the schema exactly as a template write is, and by the
		// same code — an adjustment is not a back door around the declaration.
		resp := w.License().AdjustRaw(ct.LicenseTemplateValues{"flows": 100001})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "flows", "max")
	})

	t.Run("refuses a value of the wrong type", func(t *testing.T) {
		w := newLicensedWorld(t)

		resp := w.License().AdjustRaw(ct.LicenseTemplateValues{"sso": "yes"})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_VALUE_INVALID", "sso", "type")
	})

	t.Run("refuses a license field the schema does not declare", func(t *testing.T) {
		w := newLicensedWorld(t)

		resp := w.License().AdjustRaw(ct.LicenseTemplateValues{"seats": 12})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertFieldError(t, resp.JSON400.Errors, "LICENSE_FIELD_UNKNOWN", "seats", "")
	})

	t.Run("404 when the organization has no license", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.License().AdjustRaw(ct.LicenseTemplateValues{"flows": 800})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("does not adjust another product's license", func(t *testing.T) {
		first := newLicensedWorld(t)
		second := newLicenseWorld(t)

		resp := second.License().For(first.OrganizationID()).
			AdjustRaw(ct.LicenseTemplateValues{"flows": 800})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("EmitsWebhook", func(t *testing.T) {
		w := newLicensedWorld(t)
		sink := w.product.CaptureEvents()
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})
		sink.WaitFor("organization.license.updated", map[string]string{
			"organization_id": w.OrganizationID(),
		})
	})
}

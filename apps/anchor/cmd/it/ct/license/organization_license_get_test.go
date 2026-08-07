package license_ct_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrganizationLicense(t *testing.T) {
	t.Run("reads the effective license in one call", func(t *testing.T) {
		w := newLicensedWorld(t)

		organizationLicense := w.License().Get()

		// One call answers "what is this customer allowed": every license field,
		// its value, and where the values came from.
		assert.Equal(t, w.OrganizationID(), organizationLicense.OrganizationId)
		assert.Equal(t, w.TemplateID(), organizationLicense.TemplateId)
		assert.NotZero(t, organizationLicense.InstantiatedAt)
		assertValues(t, organizationLicense.Values, validTemplateValues())
	})

	t.Run("404 when the organization has no license", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.License().GetRaw()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("404 when the product has no organization with that identifier", func(t *testing.T) {
		w := newLicensedWorld(t)

		resp := w.License().For(missingOrganizationID()).GetRaw()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("does not read another product's license", func(t *testing.T) {
		first := newLicensedWorld(t)
		second := newLicenseWorld(t)

		resp := second.License().For(first.OrganizationID()).GetRaw()
		require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

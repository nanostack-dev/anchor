package license_ct_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstantiateOrganizationLicense(t *testing.T) {
	t.Run("copies the template's values onto the organization", func(t *testing.T) {
		w := newLicenseWorld(t)

		before := time.Now().Add(-time.Second)
		organizationLicense := w.License().Instantiate(w.TemplateID())

		assert.NotEmpty(t, organizationLicense.Id)
		assert.Equal(t, w.productID(), organizationLicense.ProductId)
		assert.Equal(t, w.OrganizationID(), organizationLicense.OrganizationId)
		assertValues(t, organizationLicense.Values, validTemplateValues())

		// Provenance: which template this organization was sold, and when.
		assert.Equal(t, w.TemplateID(), organizationLicense.TemplateId)
		assert.WithinRange(t, organizationLicense.InstantiatedAt, before, time.Now().Add(time.Second))
	})

	t.Run("carries a value for every declared field", func(t *testing.T) {
		w := newLicenseWorld(t)

		organizationLicense := w.License().Instantiate(w.TemplateID())

		// A consuming product reads this and takes it at face value. Every
		// declared license field is set, so nothing downstream has to decide what
		// an absent one would have granted.
		assert.Len(t, organizationLicense.Values, len(templateSchemaFields()))
		for _, declared := range templateSchemaFields() {
			assert.Contains(t, organizationLicense.Values, declared.Name)
		}
	})

	t.Run("refuses a second license for the same organization", func(t *testing.T) {
		w := newLicensedWorld(t)

		resp := w.License().InstantiateRaw(w.TemplateID())
		require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON409)
		assertAPIError(t, resp.JSON409.Errors, "ORGANIZATION_LICENSE_EXISTS")
	})

	t.Run("400 when the product has no template with that identifier", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.License().InstantiateRaw(missingTemplateID())
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertAPIError(t, resp.JSON400.Errors, "LICENSE_TEMPLATE_NOT_FOUND_IN_REQUEST")
	})

	t.Run("404 when the product has no organization with that identifier", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.License().For(missingOrganizationID()).InstantiateRaw(w.TemplateID())
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("404 for another product's organization", func(t *testing.T) {
		first := newLicenseWorld(t)
		second := newLicenseWorld(t)

		// An organization that exists but belongs elsewhere is the same answer as
		// one that does not exist. From this product's side the two are the same
		// thing, and saying which would leak that the identifier is real.
		resp := first.License().For(second.OrganizationID()).InstantiateRaw(first.TemplateID())
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("two organizations on one template hold separate licenses", func(t *testing.T) {
		w := newLicensedWorld(t)
		first := w.License().Get()

		second := w.License().For(w.NewOrganization()).Instantiate(w.TemplateID())

		// Same values, separate records — which is what makes adjusting one of
		// them a local change rather than a change to the tier.
		assert.NotEqual(t, first.Id, second.Id)
		assert.Equal(t, first.TemplateId, second.TemplateId)
		assertValues(t, second.Values, first.Values)
	})
}

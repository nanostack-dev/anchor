package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The license-to-template direction stays isolated. The other direction is
// deliberately not — see organization_license_template_sync_test.go.
func TestOrganizationLicenseIsolation(t *testing.T) {
	t.Run("adjusting a license leaves the template unchanged", func(t *testing.T) {
		w := newLicensedWorld(t)

		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})

		// One customer's bespoke arrangement is not a change to the tier.
		assertValues(t, w.Template().Read().Values, validTemplateValues())
	})

	t.Run("adjusting one organization leaves another on the same template alone", func(t *testing.T) {
		w := newLicensedWorld(t)
		neighbour := w.License().For(w.NewOrganization())
		neighbour.Instantiate(w.TemplateID())

		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})

		assertValues(t, neighbour.Get().Values, validTemplateValues())
	})

	t.Run("archiving the template leaves the license readable", func(t *testing.T) {
		w := newLicensedWorld(t)
		templateID := w.TemplateID()

		w.Template().Archive()

		organizationLicense := w.License().Get()

		// The provenance survives the offer it names. "This customer was sold
		// that tier" does not stop being true when the tier is withdrawn.
		assert.Equal(t, templateID, organizationLicense.TemplateId)
		assertValues(t, organizationLicense.Values, validTemplateValues())
	})

	t.Run("deleting the product takes the template and the license with it", func(t *testing.T) {
		w := newLicensedWorld(t)

		// organization_licenses.template_id is a real foreign key now, and a
		// product delete cascades to both sides of it. This proves the cascade
		// still completes rather than tripping the new constraint.
		resp, err := w.client().DeleteProductWithResponse(context.Background(), w.productID())
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
	})

	t.Run("a withdrawn tier cannot be sold to anyone else", func(t *testing.T) {
		w := newLicensedWorld(t)
		newcomer := w.License().For(w.NewOrganization())

		w.Template().Archive()

		resp := newcomer.InstantiateRaw(w.TemplateID())
		require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON409)
		assertAPIError(t, resp.JSON409.Errors, "LICENSE_TEMPLATE_ARCHIVED")

		// The organization already on it keeps what it holds.
		assertValues(t, w.License().Get().Values, validTemplateValues())
	})
}

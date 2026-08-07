package license_ct_test

import (
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
)

// TestOrganizationLicenseIsolation pins the property the whole copy design
// exists for: a template and the licenses stamped from it stop affecting each
// other the moment the copy is taken. It is the reason templates need no
// versioning and per-organization deviation needs no override layer.
func TestOrganizationLicenseIsolation(t *testing.T) {
	t.Run("editing the template leaves an instantiated license unchanged", func(t *testing.T) {
		w := newLicensedWorld(t)

		w.Template().ReplaceValues(ct.LicenseTemplateValues{
			"flows": 50, "sso": false, "support_tier": "basic", "region": "eu-west",
		})

		// A live customer cannot be silently downgraded by a pricing edit.
		assertValues(t, w.License().Get().Values, validTemplateValues())
	})

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

	t.Run("deleting the template leaves the license readable", func(t *testing.T) {
		w := newLicensedWorld(t)
		templateID := w.TemplateID()

		w.Template().Delete()

		organizationLicense := w.License().Get()

		// The provenance survives the template it names. "This customer was sold
		// that tier" does not stop being true when the tier is withdrawn.
		assert.Equal(t, templateID, organizationLicense.TemplateId)
		assertValues(t, organizationLicense.Values, validTemplateValues())
	})
}

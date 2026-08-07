package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrganizationLicenseIsolation pins the property the whole copy design
// exists for: a template and the licenses stamped from it stop affecting each
// other the moment the copy is taken. It is the reason templates need no
// versioning and per-organization deviation needs no override layer.
func TestOrganizationLicenseIsolation(t *testing.T) {
	t.Run("editing the template leaves an instantiated license unchanged", func(t *testing.T) {
		lc, template := licensedOrganization(t)

		replaced := ct.LicenseTemplateValues{
			"flows": 50, "sso": false, "support_tier": "basic", "region": "eu-west",
		}
		update, err := lc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			lc.product.ProductID,
			template.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Values: &replaced},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, update.StatusCode(), string(update.Body))

		read, err := lc.product.OwnerAuthenticatedClient().GetOrganizationLicenseWithResponse(
			context.Background(), lc.product.ProductID, lc.organizationID,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, read.StatusCode(), string(read.Body))
		require.NotNil(t, read.JSON200)

		// A live customer cannot be silently downgraded by a pricing edit.
		assert.InDelta(t, 500.0, read.JSON200.Values["flows"], 0)
		assert.Equal(t, true, read.JSON200.Values["sso"])
		assert.Equal(t, "priority", read.JSON200.Values["support_tier"])
		assert.Equal(t, "ca-central", read.JSON200.Values["region"])
	})

	t.Run("adjusting a license leaves the template unchanged", func(t *testing.T) {
		lc, template := licensedOrganization(t)

		require.Equal(t, http.StatusOK, adjust(t, lc, ct.LicenseTemplateValues{"flows": 800}).StatusCode())

		read, err := lc.product.OwnerAuthenticatedClient().GetLicenseTemplateWithResponse(
			context.Background(), lc.product.ProductID, template.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, read.StatusCode(), string(read.Body))
		require.NotNil(t, read.JSON200)

		// One customer's bespoke arrangement is not a change to the tier.
		assert.InDelta(t, 500.0, read.JSON200.Values["flows"], 0)
	})

	t.Run("adjusting one organization leaves another on the same template alone", func(t *testing.T) {
		lc := newOrganizationLicenseCtx(t)
		template := createTemplate(t, lc.testCtx, uniqueTemplateName(), validTemplateValues())
		instantiateLicense(t, lc, template.Id)

		neighbour := licenseCtx{testCtx: lc.testCtx, organizationID: newOrganization(t, lc)}
		instantiateLicense(t, neighbour, template.Id)

		require.Equal(t, http.StatusOK, adjust(t, lc, ct.LicenseTemplateValues{"flows": 800}).StatusCode())

		read, err := lc.product.OwnerAuthenticatedClient().GetOrganizationLicenseWithResponse(
			context.Background(), lc.product.ProductID, neighbour.organizationID,
		)
		require.NoError(t, err)
		require.NotNil(t, read.JSON200)
		assert.InDelta(t, 500.0, read.JSON200.Values["flows"], 0)
	})

	t.Run("deleting the template leaves the license readable", func(t *testing.T) {
		lc, template := licensedOrganization(t)

		del, err := lc.product.OwnerAuthenticatedClient().DeleteLicenseTemplateWithResponse(
			context.Background(), lc.product.ProductID, template.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, del.StatusCode(), string(del.Body))

		read, err := lc.product.OwnerAuthenticatedClient().GetOrganizationLicenseWithResponse(
			context.Background(), lc.product.ProductID, lc.organizationID,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, read.StatusCode(), string(read.Body))
		require.NotNil(t, read.JSON200)

		// The provenance survives the template it names. "This customer was sold
		// that tier" does not stop being true when the tier is withdrawn.
		assert.Equal(t, template.Id, read.JSON200.TemplateId)
		assert.InDelta(t, 500.0, read.JSON200.Values["flows"], 0)
	})
}

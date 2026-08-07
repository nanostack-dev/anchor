package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrganizationLicense(t *testing.T) {
	t.Run("reads the effective license in one call", func(t *testing.T) {
		lc, template := licensedOrganization(t)

		resp, err := lc.product.OwnerAuthenticatedClient().GetOrganizationLicenseWithResponse(
			context.Background(), lc.product.ProductID, lc.organizationID,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		// One call answers "what is this customer allowed": every license field,
		// its value, and where the values came from.
		assert.Equal(t, lc.organizationID, resp.JSON200.OrganizationId)
		assert.Equal(t, template.Id, resp.JSON200.TemplateId)
		assert.NotZero(t, resp.JSON200.InstantiatedAt)
		assert.InDelta(t, 500.0, resp.JSON200.Values["flows"], 0)
		assert.Equal(t, true, resp.JSON200.Values["sso"])
		assert.Equal(t, "priority", resp.JSON200.Values["support_tier"])
		assert.Equal(t, "ca-central", resp.JSON200.Values["region"])
	})

	t.Run("404 when the organization has no license", func(t *testing.T) {
		lc := newOrganizationLicenseCtx(t)

		resp, err := lc.product.OwnerAuthenticatedClient().GetOrganizationLicenseWithResponse(
			context.Background(), lc.product.ProductID, lc.organizationID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("404 when the product has no organization with that identifier", func(t *testing.T) {
		lc := newOrganizationLicenseCtx(t)

		resp, err := lc.product.OwnerAuthenticatedClient().GetOrganizationLicenseWithResponse(
			context.Background(), lc.product.ProductID, missingOrganizationID(),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("does not read another product's license", func(t *testing.T) {
		first, _ := licensedOrganization(t)
		second := newOrganizationLicenseCtx(t)

		resp, err := second.product.OwnerAuthenticatedClient().GetOrganizationLicenseWithResponse(
			context.Background(), second.product.ProductID, first.organizationID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

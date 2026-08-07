package license_ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstantiateOrganizationLicense(t *testing.T) {
	t.Run("copies the template's values onto the organization", func(t *testing.T) {
		lc := newOrganizationLicenseCtx(t)
		template := createTemplate(t, lc.testCtx, uniqueTemplateName(), validTemplateValues())

		before := time.Now().Add(-time.Second)
		organizationLicense := instantiateLicense(t, lc, template.Id)

		assert.NotEmpty(t, organizationLicense.Id)
		assert.Equal(t, lc.product.ProductID, organizationLicense.ProductId)
		assert.Equal(t, lc.organizationID, organizationLicense.OrganizationId)

		assert.InDelta(t, 500.0, organizationLicense.Values["flows"], 0)
		assert.Equal(t, true, organizationLicense.Values["sso"])
		assert.Equal(t, "priority", organizationLicense.Values["support_tier"])
		assert.Equal(t, "ca-central", organizationLicense.Values["region"])

		// Provenance: which template this organization was sold, and when.
		assert.Equal(t, template.Id, organizationLicense.TemplateId)
		assert.WithinRange(t, organizationLicense.InstantiatedAt, before, time.Now().Add(time.Second))
	})

	t.Run("carries a value for every declared field", func(t *testing.T) {
		lc := newOrganizationLicenseCtx(t)
		template := createTemplate(t, lc.testCtx, uniqueTemplateName(), validTemplateValues())

		organizationLicense := instantiateLicense(t, lc, template.Id)

		// A consuming product reads this and takes it at face value. Every
		// declared license field is set, so nothing downstream has to decide what
		// an absent one would have granted.
		assert.Len(t, organizationLicense.Values, len(templateSchemaFields()))
		for _, declared := range templateSchemaFields() {
			assert.Contains(t, organizationLicense.Values, declared.Name)
		}
	})

	t.Run("refuses a second license for the same organization", func(t *testing.T) {
		lc, template := licensedOrganization(t)

		resp, err := lc.product.OwnerAuthenticatedClient().InstantiateOrganizationLicenseWithResponse(
			context.Background(),
			lc.product.ProductID,
			lc.organizationID,
			ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: template.Id},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertAPIError(t, resp.JSON400.Errors, "ORGANIZATION_LICENSE_EXISTS")
	})

	t.Run("404 when the product has no template with that identifier", func(t *testing.T) {
		lc := newOrganizationLicenseCtx(t)

		resp, err := lc.product.OwnerAuthenticatedClient().InstantiateOrganizationLicenseWithResponse(
			context.Background(),
			lc.product.ProductID,
			lc.organizationID,
			ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: missingTemplateID()},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("404 when the product has no organization with that identifier", func(t *testing.T) {
		lc := newOrganizationLicenseCtx(t)
		template := createTemplate(t, lc.testCtx, uniqueTemplateName(), validTemplateValues())

		resp, err := lc.product.OwnerAuthenticatedClient().InstantiateOrganizationLicenseWithResponse(
			context.Background(),
			lc.product.ProductID,
			missingOrganizationID(),
			ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: template.Id},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("404 for another product's organization", func(t *testing.T) {
		first := newOrganizationLicenseCtx(t)
		second := newOrganizationLicenseCtx(t)
		template := createTemplate(t, first.testCtx, uniqueTemplateName(), validTemplateValues())

		// An organization that exists but belongs elsewhere is the same answer as
		// one that does not exist. From this product's side the two are the same
		// thing, and saying which would leak that the identifier is real.
		resp, err := first.product.OwnerAuthenticatedClient().InstantiateOrganizationLicenseWithResponse(
			context.Background(),
			first.product.ProductID,
			second.organizationID,
			ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: template.Id},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("two organizations on one template hold separate licenses", func(t *testing.T) {
		lc := newOrganizationLicenseCtx(t)
		template := createTemplate(t, lc.testCtx, uniqueTemplateName(), validTemplateValues())
		first := instantiateLicense(t, lc, template.Id)

		secondOrganization := newOrganization(t, lc)
		resp, err := lc.product.OwnerAuthenticatedClient().InstantiateOrganizationLicenseWithResponse(
			context.Background(),
			lc.product.ProductID,
			secondOrganization,
			ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: template.Id},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON201)

		// Same values, separate records — which is what makes adjusting one of
		// them a local change rather than a change to the tier.
		assert.NotEqual(t, first.Id, resp.JSON201.Id)
		assert.Equal(t, first.TemplateId, resp.JSON201.TemplateId)
		assert.Equal(t, first.Values, resp.JSON201.Values)
	})
}

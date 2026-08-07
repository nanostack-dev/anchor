package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrganizationLicenseScopes checks each route against a Product API key
// holding every other organization license scope. The route security matrix
// already proves the contract declares security; this proves the scopes are
// genuinely distinct, so a read-only key cannot raise a customer's limits.
func TestOrganizationLicenseScopes(t *testing.T) {
	t.Run("read scope cannot write", func(t *testing.T) {
		lc, template := licensedOrganization(t)
		readOnly, _ := lc.product.CreateAPIKeyClientWithScopes([]string{"organization_license:read"})

		instantiate, err := readOnly.InstantiateOrganizationLicenseWithResponse(
			context.Background(),
			lc.product.ProductID,
			newOrganization(t, lc),
			ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: template.Id},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, instantiate.StatusCode())

		adjustResp, err := readOnly.AdjustOrganizationLicenseWithResponse(
			context.Background(),
			lc.product.ProductID,
			lc.organizationID,
			ct.AdjustOrganizationLicenseJSONRequestBody{
				Values: ct.LicenseTemplateValues{"flows": 800},
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, adjustResp.StatusCode())
	})

	t.Run("write scopes cannot read", func(t *testing.T) {
		lc := newOrganizationLicenseCtx(t)
		template := createTemplate(t, lc.testCtx, uniqueTemplateName(), validTemplateValues())
		writeOnly, _ := lc.product.CreateAPIKeyClientWithScopes(
			[]string{"organization_license:create", "organization_license:update"},
		)

		instantiate, err := writeOnly.InstantiateOrganizationLicenseWithResponse(
			context.Background(),
			lc.product.ProductID,
			lc.organizationID,
			ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: template.Id},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, instantiate.StatusCode(), string(instantiate.Body))

		read, err := writeOnly.GetOrganizationLicenseWithResponse(
			context.Background(), lc.product.ProductID, lc.organizationID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, read.StatusCode())

		diff, err := writeOnly.GetOrganizationLicenseDiffWithResponse(
			context.Background(), lc.product.ProductID, lc.organizationID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, diff.StatusCode())
	})

	t.Run("a license template scope does not reach organization licenses", func(t *testing.T) {
		lc, template := licensedOrganization(t)
		// The two resources are separate entries in the permission catalog, so a
		// key trusted to decide what a tier grants is not thereby trusted to
		// change what one customer holds.
		templateOnly, _ := lc.product.CreateAPIKeyClientWithScopes(
			[]string{"license_template:read", "license_template:create", "license_template:update"},
		)

		read, err := templateOnly.GetOrganizationLicenseWithResponse(
			context.Background(), lc.product.ProductID, lc.organizationID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, read.StatusCode())

		instantiate, err := templateOnly.InstantiateOrganizationLicenseWithResponse(
			context.Background(),
			lc.product.ProductID,
			newOrganization(t, lc),
			ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: template.Id},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, instantiate.StatusCode())
	})
}

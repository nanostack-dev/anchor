package license_ct_test

import (
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
		w := newLicensedWorld(t)
		readOnly, _ := w.product.CreateAPIKeyClientWithScopes([]string{"organization_license:read"})
		license := w.License().As(readOnly)

		instantiate := license.For(w.NewOrganization()).InstantiateRaw(w.TemplateID())
		assert.Equal(t, http.StatusForbidden, instantiate.StatusCode())

		adjusted := license.AdjustRaw(ct.LicenseTemplateValues{"flows": 800})
		assert.Equal(t, http.StatusForbidden, adjusted.StatusCode())
	})

	t.Run("write scopes cannot read", func(t *testing.T) {
		w := newLicenseWorld(t)
		writeOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{"organization_license:create", "organization_license:update"},
		)
		license := w.License().As(writeOnly)

		instantiate := license.InstantiateRaw(w.TemplateID())
		require.Equal(t, http.StatusCreated, instantiate.StatusCode(), string(instantiate.Body))

		assert.Equal(t, http.StatusForbidden, license.GetRaw().StatusCode())
		assert.Equal(t, http.StatusForbidden, license.DiffRaw().StatusCode())
	})

	t.Run("a license template scope does not reach organization licenses", func(t *testing.T) {
		w := newLicensedWorld(t)
		// The two resources are separate entries in the permission catalog, so a
		// key trusted to decide what a tier grants is not thereby trusted to
		// change what one customer holds.
		templateOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{"license_template:read", "license_template:create", "license_template:update"},
		)
		license := w.License().As(templateOnly)

		assert.Equal(t, http.StatusForbidden, license.GetRaw().StatusCode())

		instantiate := license.For(w.NewOrganization()).InstantiateRaw(w.TemplateID())
		assert.Equal(t, http.StatusForbidden, instantiate.StatusCode())
	})
}

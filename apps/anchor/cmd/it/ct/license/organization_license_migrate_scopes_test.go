package license_ct_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrateOrganizationLicensesScopes proves the migrate scope is genuinely
// its own. A migration addresses every organization the product has, chosen by
// a selector rather than named in the path, and it restamps provenance that no
// other write can touch — so a key trusted to adjust one customer is not
// thereby trusted to move the whole book.
func TestMigrateOrganizationLicensesScopes(t *testing.T) {
	t.Run("the organization license write scopes do not reach it", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())
		writeOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{"organization_license:create", "organization_license:update"},
		)

		resp := w.Migration().As(writeOnly).RunRaw(migrateTo(pro.Id, w.OrganizationID()))
		assert.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("the migrate scope alone is enough", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())
		migrateOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{"organization_license:migrate"},
		)

		resp := w.Migration().As(migrateOnly).RunRaw(migrateTo(pro.Id, w.OrganizationID()))
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	})

	t.Run("it does not reach the per-organization routes", func(t *testing.T) {
		w := newLicensedWorld(t)
		migrateOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{"organization_license:migrate"},
		)
		license := w.License().As(migrateOnly)

		assert.Equal(t, http.StatusForbidden, license.GetRaw().StatusCode())
		assert.Equal(t, http.StatusForbidden, license.InstantiateRaw(w.TemplateID()).StatusCode())
	})
}

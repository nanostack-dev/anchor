package license_ct_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrateOrganizationLicensesScopes proves the migrate route shares its
// scope with the per-organization adjust route rather than having one of its
// own: granting and moving a license are both, from a permissions standpoint,
// changing what an organization's license says. See
// docs/adr/0015-migrate-grants-a-first-license.md.
func TestMigrateOrganizationLicensesScopes(t *testing.T) {
	t.Run("read and create alone do not reach it", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())
		readCreateOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{"organization_license:read", "organization_license:create"},
		)

		resp := w.Migration().As(readCreateOnly).RunRaw(migrateTo(pro.Id, w.OrganizationID()))
		assert.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("the update scope is enough", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())
		updateOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{"organization_license:update"},
		)

		resp := w.Migration().As(updateOnly).RunRaw(migrateTo(pro.Id, w.OrganizationID()))
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	})

	t.Run("the same scope also grants a first license", func(t *testing.T) {
		w := newLicensedWorld(t)
		unlicensed := w.NewOrganization()
		pro := w.NewTemplate(proValues())
		updateOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{"organization_license:update"},
		)

		resp := w.Migration().As(updateOnly).RunRaw(migrateTo(pro.Id, unlicensed))
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	})
}

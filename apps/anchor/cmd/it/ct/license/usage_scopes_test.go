package license_ct_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageScopes(t *testing.T) {
	t.Run("a license scope cannot report usage", func(t *testing.T) {
		w := newLicensedWorld(t)
		licenseOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{
				"organization_license:read",
				"organization_license:create",
				"organization_license:update",
			},
		)

		resp := w.Usage().As(licenseOnly).ReportRaw(gauge("flows", 37))

		assert.Equal(t, http.StatusForbidden, resp.StatusCode(), string(resp.Body))
	})

	t.Run("the usage scope cannot read the license", func(t *testing.T) {
		w := newLicensedWorld(t)
		usageOnly, _ := w.product.CreateAPIKeyClientWithScopes([]string{"license_usage:create"})

		report := w.Usage().As(usageOnly).ReportRaw(gauge("flows", 37))
		require.Equal(t, http.StatusCreated, report.StatusCode(), string(report.Body))

		assert.Equal(t, http.StatusForbidden, w.License().As(usageOnly).GetRaw().StatusCode())
		assert.Equal(t, http.StatusForbidden, w.License().As(usageOnly).DiffRaw().StatusCode())
	})

	t.Run("a license template scope does not reach usage", func(t *testing.T) {
		w := newLicenseWorld(t)
		templateOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{"license_template:read", "license_template:create", "license_template:update"},
		)

		resp := w.Usage().As(templateOnly).ReportRaw(gauge("flows", 37))

		assert.Equal(t, http.StatusForbidden, resp.StatusCode(), string(resp.Body))
	})
}

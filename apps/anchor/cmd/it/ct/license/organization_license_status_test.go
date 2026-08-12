package license_ct_test

import (
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireFieldUsage pulls worldLimitKey's usage entry out of a license read,
// failing the test if the read carries no usage map or no entry for it.
func requireFieldUsage(t *testing.T, license ct.OrganizationLicenseResponse) ct.LicenseFieldUsageResponse {
	t.Helper()
	require.NotNil(t, license.Usage, "license read carries no usage map")
	usage, ok := (*license.Usage)[worldLimitKey]
	require.True(t, ok, "no usage entry for field %q", worldLimitKey)
	return usage
}

// TestOrganizationLicenseUsageStatus covers every derived status the license
// read can carry for a limit field, a limit adjustment that flips a
// compliant organization to exceeded, and the two cache-consistency
// guarantees the read makes.
func TestOrganizationLicenseUsageStatus(t *testing.T) {
	t.Run("within_limit: latest usage under the limit", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.Usage().Report(gauge("flows", 400))

		usage := requireFieldUsage(t, w.License().Get())

		assert.Equal(t, ct.WithinLimit, usage.Status)
		assert.InDelta(t, 500.0, usage.Limit, 0)
		require.NotNil(t, usage.Usage)
		assert.InDelta(t, 400.0, *usage.Usage, 0)
		require.NotNil(t, usage.LastReportedAt)
	})

	t.Run("at_limit: latest usage equal to the limit", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.Usage().Report(gauge("flows", 500))

		usage := requireFieldUsage(t, w.License().Get())

		assert.Equal(t, ct.AtLimit, usage.Status)
	})

	t.Run("exceeded: latest usage past the limit", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.Usage().Report(gauge("flows", 9000))

		usage := requireFieldUsage(t, w.License().Get())

		assert.Equal(t, ct.Exceeded, usage.Status)
		require.NotNil(t, usage.Usage)
		assert.InDelta(t, 9000.0, *usage.Usage, 0)
	})

	t.Run("stale: nothing has ever been reported", func(t *testing.T) {
		w := newLicensedWorld(t)

		usage := requireFieldUsage(t, w.License().Get())

		assert.Equal(t, ct.Stale, usage.Status)
		assert.Nil(t, usage.Usage)
		assert.Nil(t, usage.LastReportedAt)
	})

	t.Run("a limit adjusted so a previously-compliant organization becomes exceeded", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.Usage().Report(gauge("flows", 400))
		before := requireFieldUsage(t, w.License().Get())
		require.Equal(t, ct.WithinLimit, before.Status)

		w.License().Adjust(ct.LicenseTemplateValues{"flows": 300})

		after := requireFieldUsage(t, w.License().Get())
		assert.Equal(t, ct.Exceeded, after.Status)
		assert.InDelta(t, 300.0, after.Limit, 0)
	})

	t.Run("usage arriving is visible on the next read without a license write", func(t *testing.T) {
		w := newLicensedWorld(t)
		// Populate the license cache first, mirroring a consumer's hot-path
		// call landing before any usage exists.
		first := requireFieldUsage(t, w.License().Get())
		require.Equal(t, ct.Stale, first.Status)

		w.Usage().Report(gauge("flows", 9000))

		after := requireFieldUsage(t, w.License().Get())
		assert.Equal(t, ct.Exceeded, after.Status)
	})

	t.Run("the cache is evicted when the license is adjusted", func(t *testing.T) {
		w := newLicensedWorld(t)
		first := w.License().Get()
		assertValues(t, first.Values, validTemplateValues())

		w.License().Adjust(ct.LicenseTemplateValues{"flows": 999})

		second := w.License().Get()
		assertValues(t, second.Values, ct.LicenseTemplateValues{
			"flows": 999, "sso": true, "support_tier": "priority", "region": "ca-central",
		})
	})
}

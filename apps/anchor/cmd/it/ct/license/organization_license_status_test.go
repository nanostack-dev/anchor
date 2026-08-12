package license_ct_test

import (
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// statusSchemaFields declares one limit with a one-hour expected reporting
// interval — isolated from templateSchemaFields so this suite can backdate
// observations in the database without touching the fixture every other
// license test builds against.
func statusSchemaFields() []ct.LicenseFieldDeclaration {
	return []ct.LicenseFieldDeclaration{
		{
			Name:                             "flows",
			Type:                             ct.LicenseFieldTypeLIMIT,
			Rules:                            limitRules(0, 100000),
			ExpectedReportingIntervalSeconds: new(3600),
		},
	}
}

// newStatusWorld builds a product declaring statusSchemaFields, with one
// organization already licensed at flows=500.
func newStatusWorld(t *testing.T) *licenseWorld {
	t.Helper()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true}).
		Product(itdsl.ProductOpts{Alias: "p", TenantAlias: "t"}).
		LicenseSchema(itdsl.LicenseSchemaOpts{
			Alias: "s", ProductAlias: "p", Fields: statusSchemaFields(),
		}).
		ProductOrganization(itdsl.ProductOrganizationOpts{
			Alias: worldOrganizationAlias, ProductAlias: "p",
		}).
		LicenseTemplate(itdsl.LicenseTemplateOpts{
			Alias: worldTemplateAlias, ProductAlias: "p", Name: uniqueTemplateName(),
			Values: ct.LicenseTemplateValues{"flows": 500},
		}).
		Build()

	world := &licenseWorld{
		t: t, state: state, product: state.Product("p"), tenantID: state.Tenant("t").ID,
	}
	world.License().Instantiate(world.TemplateID())
	return world
}

// requireFieldUsage pulls worldLimitKey's usage entry out of a license read,
// failing the test if the read carries no usage map or no entry for it. Every
// world this suite builds — newStatusWorld included — declares its one limit
// under that same name.
func requireFieldUsage(t *testing.T, license ct.OrganizationLicenseResponse) ct.LicenseFieldUsageResponse {
	t.Helper()
	require.NotNil(t, license.Usage, "license read carries no usage map")
	usage, ok := (*license.Usage)[worldLimitKey]
	require.True(t, ok, "no usage entry for field %q", worldLimitKey)
	return usage
}

// backdate rewrites an observation's observed_at directly in storage, so a
// staleness test controls elapsed time exactly instead of sleeping.
func backdate(t *testing.T, observationID string, at time.Time) {
	t.Helper()
	_, err := testDB.Exec(`UPDATE usage_observations SET observed_at = $1 WHERE id = $2`, at, observationID)
	require.NoError(t, err)
}

// TestOrganizationLicenseUsageStatus covers every derived status the license
// read can carry for a limit field, including both shapes of stale, a limit
// adjustment that flips a compliant organization to exceeded, and the two
// cache-consistency guarantees the read makes.
func TestOrganizationLicenseUsageStatus(t *testing.T) {
	t.Run("within_limit: latest usage under the limit", func(t *testing.T) {
		w := newStatusWorld(t)
		w.Usage().Report(gauge("flows", 400))

		usage := requireFieldUsage(t, w.License().Get())

		assert.Equal(t, ct.WithinLimit, usage.Status)
		assert.InDelta(t, 500.0, usage.Limit, 0)
		require.NotNil(t, usage.Usage)
		assert.InDelta(t, 400.0, *usage.Usage, 0)
		require.NotNil(t, usage.LastReportedAt)
	})

	t.Run("at_limit: latest usage equal to the limit", func(t *testing.T) {
		w := newStatusWorld(t)
		w.Usage().Report(gauge("flows", 500))

		usage := requireFieldUsage(t, w.License().Get())

		assert.Equal(t, ct.AtLimit, usage.Status)
	})

	t.Run("exceeded: latest usage past the limit", func(t *testing.T) {
		w := newStatusWorld(t)
		w.Usage().Report(gauge("flows", 9000))

		usage := requireFieldUsage(t, w.License().Get())

		assert.Equal(t, ct.Exceeded, usage.Status)
		require.NotNil(t, usage.Usage)
		assert.InDelta(t, 9000.0, *usage.Usage, 0)
	})

	t.Run("stale: nothing has ever been reported", func(t *testing.T) {
		w := newStatusWorld(t)

		usage := requireFieldUsage(t, w.License().Get())

		assert.Equal(t, ct.Stale, usage.Status)
		assert.Nil(t, usage.Usage)
		assert.Nil(t, usage.LastReportedAt)
	})

	t.Run("stale: latest observation older than the declared expected reporting interval", func(t *testing.T) {
		w := newStatusWorld(t)
		observation := w.Usage().Report(gauge("flows", 400))
		backdate(t, observation.Id, time.Now().Add(-2*time.Hour))

		usage := requireFieldUsage(t, w.License().Get())

		assert.Equal(t, ct.Stale, usage.Status)
		// Stale from age still reports the number that went stale, not a null:
		// a consumer deciding whether to trust it needs to see what it is.
		require.NotNil(t, usage.Usage)
		assert.InDelta(t, 400.0, *usage.Usage, 0)
		require.NotNil(t, usage.LastReportedAt)
	})

	t.Run("a limit adjusted so a previously-compliant organization becomes exceeded", func(t *testing.T) {
		w := newStatusWorld(t)
		w.Usage().Report(gauge("flows", 400))
		before := requireFieldUsage(t, w.License().Get())
		require.Equal(t, ct.WithinLimit, before.Status)

		w.License().Adjust(ct.LicenseTemplateValues{"flows": 300})

		after := requireFieldUsage(t, w.License().Get())
		assert.Equal(t, ct.Exceeded, after.Status)
		assert.InDelta(t, 300.0, after.Limit, 0)
	})

	t.Run("usage arriving is visible on the next read without a license write", func(t *testing.T) {
		w := newStatusWorld(t)
		// Populate the license cache first, mirroring a consumer's hot-path
		// call landing before any usage exists.
		first := requireFieldUsage(t, w.License().Get())
		require.Equal(t, ct.Stale, first.Status)

		w.Usage().Report(gauge("flows", 9000))

		after := requireFieldUsage(t, w.License().Get())
		assert.Equal(t, ct.Exceeded, after.Status)
	})

	t.Run("the cache is evicted when the license is adjusted", func(t *testing.T) {
		w := newStatusWorld(t)
		first := w.License().Get()
		assertValues(t, first.Values, ct.LicenseTemplateValues{"flows": 500})

		w.License().Adjust(ct.LicenseTemplateValues{"flows": 999})

		second := w.License().Get()
		assertValues(t, second.Values, ct.LicenseTemplateValues{"flows": 999})
	})
}

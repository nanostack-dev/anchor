package license_ct_test

import (
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func markLegacyLicense(t *testing.T, w *licenseWorld) {
	t.Helper()
	_, err := testDB.ExecContext(
		t.Context(),
		`UPDATE organization_licenses SET adjusted_fields = 'null'::jsonb WHERE organization_id = $1`,
		w.OrganizationID(),
	)
	require.NoError(t, err)
}

func TestLegacyAdjustmentsSurviveFirstTemplateSync(t *testing.T) {
	t.Run("historical adjustments remain pinned even when equal to the template", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800, "sso": false})
		w.License().Adjust(ct.LicenseTemplateValues{"sso": true})
		markLegacyLicense(t, w)
		require.NoError(t, adjustmentBackfill.Run(t.Context()))
		require.NoError(t, adjustmentBackfill.Run(t.Context()))
		w.Template().ReplaceValues(ct.LicenseTemplateValues{
			"flows": 50, "sso": false, "support_tier": "basic", "region": "eu-west",
		})
		synced := waitForLicenseValues(t, w.License(), ct.LicenseTemplateValues{
			"flows": 800, "sso": true, "support_tier": "basic", "region": "eu-west",
		})
		assert.Equal(t, []string{"flows", "sso"}, synced.AdjustedFields)
		assert.Equal(t, 1, countTemplateSyncChanges(t, w.License()))
	})

	t.Run("historyless differences are preserved while missing fields follow", func(t *testing.T) {
		w := newLicensedWorld(t)
		_, err := testDB.ExecContext(t.Context(),
			`UPDATE organization_licenses SET values_json = (values_json - 'region') || '{"flows":800}'::jsonb
			 WHERE organization_id = $1`, w.OrganizationID())
		require.NoError(t, err)
		markLegacyLicense(t, w)
		require.NoError(t, adjustmentBackfill.Run(t.Context()))
		w.Template().ReplaceValues(validTemplateValues())
		synced := waitForLicenseValues(t, w.License(), ct.LicenseTemplateValues{
			"flows": 800, "sso": true, "support_tier": "priority", "region": "ca-central",
		})
		assert.Equal(t, []string{"flows"}, synced.AdjustedFields)
	})

	t.Run("a discarded adjustment is not resurrected", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})
		w.Migration().Run(ct.OrganizationLicenseMigrationRequest{
			TemplateId: w.TemplateID(), OnDifference: new(ct.DISCARD),
			OrganizationIds: &[]string{w.OrganizationID()},
		})
		markLegacyLicense(t, w)
		require.NoError(t, adjustmentBackfill.Run(t.Context()))
		updated := ct.LicenseTemplateValues{
			"flows": 50, "sso": false, "support_tier": "basic", "region": "eu-west",
		}
		w.Template().ReplaceValues(updated)
		synced := waitForLicenseValues(t, w.License(), updated)
		assert.Empty(t, synced.AdjustedFields)
	})
}

package license_ct_test

import (
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The propagation is a durable queue job, so every assertion on its effect
// polls until the worker has caught up; templateSyncSettleTime is the wait
// before asserting that nothing happened.
const (
	templateSyncWaitTimeout = 30 * time.Second
	templateSyncWaitTick    = 250 * time.Millisecond
	templateSyncSettleTime  = 4 * time.Second
)

func waitForLicenseValues(
	t *testing.T, handle licenseHandle, expected ct.LicenseTemplateValues,
) ct.OrganizationLicenseResponse {
	t.Helper()
	var last ct.OrganizationLicenseResponse
	require.Eventuallyf(t, func() bool {
		last = handle.Get()
		return assert.ObjectsAreEqual(normaliseValues(expected), normaliseValues(last.Values))
	}, templateSyncWaitTimeout, templateSyncWaitTick,
		"license never followed its template; last read %v", &last.Values)
	return last
}

func countTemplateSyncChanges(t *testing.T, handle licenseHandle) int {
	t.Helper()
	history := handle.History()
	count := 0
	for _, change := range history.Items {
		if change.Type == ct.TEMPLATESYNCED {
			count++
		}
	}
	return count
}

func TestLicenseFollowsItsTemplate(t *testing.T) {
	t.Run("a template value update reaches an unadjusted license", func(t *testing.T) {
		w := newLicensedWorld(t)
		require.Empty(t, w.License().Get().AdjustedFields)

		updated := ct.LicenseTemplateValues{
			"flows": 50, "sso": false, "support_tier": "basic", "region": "eu-west",
		}
		w.Template().ReplaceValues(updated)

		synced := waitForLicenseValues(t, w.License(), updated)
		assert.Empty(t, synced.AdjustedFields)
		assert.Equal(t, 1, countTemplateSyncChanges(t, w.License()))
	})

	t.Run("an adjusted field survives a template update", func(t *testing.T) {
		w := newLicensedWorld(t)
		adjusted := w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})
		assert.Equal(t, []string{"flows"}, adjusted.AdjustedFields)

		w.Template().ReplaceValues(ct.LicenseTemplateValues{
			"flows": 50, "sso": false, "support_tier": "basic", "region": "eu-west",
		})

		synced := waitForLicenseValues(t, w.License(), ct.LicenseTemplateValues{
			"flows": 800, "sso": false, "support_tier": "basic", "region": "eu-west",
		})
		assert.Equal(t, []string{"flows"}, synced.AdjustedFields)
	})

	t.Run("a license already holding the resolved values is left untouched", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{
			"flows": 800, "sso": false, "support_tier": "basic", "region": "eu-west",
		})

		// Every field adjusted: the sync resolves to what is already held.
		w.Template().ReplaceValues(ct.LicenseTemplateValues{
			"flows": 50, "sso": true, "support_tier": "priority", "region": "us-east",
		})

		time.Sleep(templateSyncSettleTime)
		assert.Equal(t, 0, countTemplateSyncChanges(t, w.License()))
		assertValues(t, w.License().Get().Values, ct.LicenseTemplateValues{
			"flows": 800, "sso": false, "support_tier": "basic", "region": "eu-west",
		})
	})

	t.Run("an organization on another template is left alone", func(t *testing.T) {
		w := newLicensedWorld(t)
		other := w.NewTemplate(validTemplateValues())
		neighbour := w.License().For(w.NewOrganization())
		neighbour.Instantiate(other.Id)

		updated := ct.LicenseTemplateValues{
			"flows": 50, "sso": false, "support_tier": "basic", "region": "eu-west",
		}
		w.Template().ReplaceValues(updated)

		waitForLicenseValues(t, w.License(), updated)
		assertValues(t, neighbour.Get().Values, validTemplateValues())
		assert.Equal(t, 0, countTemplateSyncChanges(t, neighbour))
	})

	t.Run("restating the stored values repairs a drifted license", func(t *testing.T) {
		w := newLicensedWorld(t)
		// Drift predating the follow rule can only be planted in the database.
		_, err := testDB.Exec(
			`UPDATE organization_licenses SET values_json = values_json - 'region'
			 WHERE organization_id = $1`,
			w.OrganizationID(),
		)
		require.NoError(t, err)

		w.Template().ReplaceValues(validTemplateValues())

		synced := waitForLicenseValues(t, w.License(), validTemplateValues())
		assert.Empty(t, synced.AdjustedFields)
		assert.Equal(t, 1, countTemplateSyncChanges(t, w.License()))
	})

	t.Run("a rename alone propagates nothing", func(t *testing.T) {
		w := newLicensedWorld(t)

		resp, err := w.client().UpdateLicenseTemplateWithResponse(
			t.Context(),
			w.productID(),
			w.TemplateID(),
			ct.UpdateLicenseTemplateJSONRequestBody{Name: new(uniqueTemplateName())},
		)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode(), string(resp.Body))

		time.Sleep(templateSyncSettleTime)
		assert.Equal(t, 0, countTemplateSyncChanges(t, w.License()))
	})
}

func TestSchemaChangeCascadesToLicenses(t *testing.T) {
	t.Run("a removed field cascades through the template onto the license", func(t *testing.T) {
		w := newLicensedWorld(t)

		fields := templateSchemaFields()
		var kept []ct.LicenseFieldDeclaration
		for _, field := range fields {
			if field.Name != "region" {
				kept = append(kept, field)
			}
		}
		w.RedeclareSchema(kept)

		templateValues := w.Template().Read().Values
		_, held := templateValues["region"]
		assert.False(t, held, "the template still carries the removed field")

		waitForLicenseValues(t, w.License(), ct.LicenseTemplateValues{
			"flows": 500, "sso": true, "support_tier": "priority",
		})
	})

	t.Run("a redeclaration that removes nothing cascades nothing", func(t *testing.T) {
		w := newLicensedWorld(t)

		w.RedeclareSchema(templateSchemaFields())

		time.Sleep(templateSyncSettleTime)
		assert.Equal(t, 0, countTemplateSyncChanges(t, w.License()))
		assertValues(t, w.License().Get().Values, validTemplateValues())
	})
}

func TestTemplateSyncRefusesAnInvalidMerge(t *testing.T) {
	w := newLicensedWorld(t)
	bystander := w.License().For(w.NewOrganization())
	bystander.Instantiate(w.TemplateID())
	w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})

	// Tighten the limit underneath the adjusted 800.
	tightened := templateSchemaFields()
	for i := range tightened {
		if tightened[i].Name == "flows" {
			tightened[i].Rules = limitRules(0, 600)
		}
	}
	w.RedeclareSchema(tightened)

	w.Template().ReplaceValues(ct.LicenseTemplateValues{
		"flows": 400, "sso": false, "support_tier": "basic", "region": "eu-west",
	})

	waitForLicenseValues(t, bystander, ct.LicenseTemplateValues{
		"flows": 400, "sso": false, "support_tier": "basic", "region": "eu-west",
	})

	// Refused whole: not half-applied, adjustment not dropped.
	refused := w.License().Get()
	assertValues(t, refused.Values, ct.LicenseTemplateValues{
		"flows": 800, "sso": true, "support_tier": "priority", "region": "ca-central",
	})
	assert.Equal(t, []string{"flows"}, refused.AdjustedFields)
	assert.Equal(t, 0, countTemplateSyncChanges(t, w.License()))
}

func TestMigrationResetsWhatFollowsATemplate(t *testing.T) {
	t.Run("discard clears the adjusted record, so the license follows again", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})

		migration := w.Migration().Run(ct.OrganizationLicenseMigrationRequest{
			TemplateId:      w.TemplateID(),
			OrganizationIds: &[]string{w.OrganizationID()},
			OnDifference:    new(ct.DISCARD),
		})
		require.Equal(t, 1, migration.Changed)
		assert.Empty(t, w.License().Get().AdjustedFields)

		updated := ct.LicenseTemplateValues{
			"flows": 50, "sso": false, "support_tier": "basic", "region": "eu-west",
		}
		w.Template().ReplaceValues(updated)

		waitForLicenseValues(t, w.License(), updated)
	})

	t.Run("carry forward keeps the pin on the new template", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})
		target := w.NewTemplate(ct.LicenseTemplateValues{
			"flows": 5000, "sso": true, "support_tier": "priority", "region": "us-east",
		})

		migration := w.Migration().Run(ct.OrganizationLicenseMigrationRequest{
			TemplateId:      target.Id,
			OrganizationIds: &[]string{w.OrganizationID()},
		})
		require.Equal(t, 1, migration.Changed)
		moved := w.License().Get()
		assert.Equal(t, []string{"flows"}, moved.AdjustedFields)

		resp, err := w.client().UpdateLicenseTemplateWithResponse(
			t.Context(),
			w.productID(),
			target.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Values: &ct.LicenseTemplateValues{
				"flows": 9000, "sso": false, "support_tier": "basic", "region": "eu-west",
			}},
		)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode(), string(resp.Body))

		synced := waitForLicenseValues(t, w.License(), ct.LicenseTemplateValues{
			"flows": 800, "sso": false, "support_tier": "basic", "region": "eu-west",
		})
		assert.Equal(t, []string{"flows"}, synced.AdjustedFields)
	})
}

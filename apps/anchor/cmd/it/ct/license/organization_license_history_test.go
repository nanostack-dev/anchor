package license_ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// backfillLicenseHistorySQL matches migration 000032. The test re-runs it
// after a raw license insert so a pre-existing grant still reads as instantiated.
const backfillLicenseHistorySQL = `
INSERT INTO organization_license_changes (
    id, platform_tenant_id, product_id, organization_id, license_id,
    change_type, template_id, new_value_json, changed_at
)
SELECT
    replace(id, 'lic_', 'lchg_'),
    platform_tenant_id,
    product_id,
    organization_id,
    id,
    'INSTANTIATED',
    template_id,
    values_json,
    instantiated_at
FROM organization_licenses
WHERE NOT EXISTS (
    SELECT 1
    FROM organization_license_changes
    WHERE organization_license_changes.license_id = organization_licenses.id
)`

func TestOrganizationLicenseHistoryRecordsInstantiation(t *testing.T) {
	t.Run("instantiating records the template it was stamped from", func(t *testing.T) {
		w := newLicenseWorld(t)

		before := time.Now().Add(-time.Second)
		organizationLicense := w.License().Instantiate(w.TemplateID())

		history := w.License().History()
		require.Equal(t, int64(1), history.Total)
		require.Len(t, history.Items, 1)

		entry := history.Items[0]
		assert.NotEmpty(t, entry.Id)
		assert.Equal(t, ct.INSTANTIATED, entry.Type)
		assert.Equal(t, w.productID(), entry.ProductId)
		assert.Equal(t, w.OrganizationID(), entry.OrganizationId)
		assert.Equal(t, organizationLicense.Id, entry.LicenseId)
		assert.Equal(t, w.TemplateID(), deref(entry.TemplateId))
		assert.WithinRange(t, entry.ChangedAt, before, time.Now().Add(time.Second))
	})

	t.Run("carries the whole set of values copied and names no field", func(t *testing.T) {
		w := newLicenseWorld(t)

		w.License().Instantiate(w.TemplateID())

		entry := w.License().History().Items[0]
		assert.Nil(t, entry.Field)
		assert.Nil(t, entry.OldValue)
		assertValues(t, newValueSet(t, entry), validTemplateValues())
	})

	t.Run("an organization with no license has an empty history", func(t *testing.T) {
		w := newLicenseWorld(t)

		// Nothing has happened to this organization yet, which is a fact rather
		// than an absence.
		history := w.License().History()
		assert.Equal(t, int64(0), history.Total)
		assert.NotNil(t, history.Items)
		assert.Empty(t, history.Items)
	})
}

func TestOrganizationLicenseHistoryRecordsAdjustment(t *testing.T) {
	t.Run("names the license field, the old value and the new", func(t *testing.T) {
		w := newLicensedWorld(t)

		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})

		entry := w.License().History().Items[0]
		assert.Equal(t, ct.ADJUSTED, entry.Type)
		assert.Equal(t, "flows", deref(entry.Field))
		assert.InDelta(t, 500.0, entry.OldValue, 0)
		assert.InDelta(t, 800.0, entry.NewValue, 0)
		// An adjustment does not move the provenance, so it names no template.
		assert.Nil(t, entry.TemplateId)
	})

	t.Run("records one entry per license field the adjustment moves", func(t *testing.T) {
		w := newLicensedWorld(t)

		w.License().Adjust(ct.LicenseTemplateValues{"flows": 900, "sso": false})

		history := w.License().History()
		require.Equal(t, int64(3), history.Total, "two adjustments and the instantiation")

		adjusted := changesByField(history.Items)
		require.Contains(t, adjusted, "flows")
		require.Contains(t, adjusted, "sso")
		assert.InDelta(t, 900.0, adjusted["flows"].NewValue, 0)
		assert.Equal(t, false, adjusted["sso"].NewValue)
		assert.Equal(t, true, adjusted["sso"].OldValue)

		// One request, one moment: the entries of a single adjustment share a
		// timestamp, so a reader is not shown a sequence that never happened.
		assert.Equal(t, adjusted["flows"].ChangedAt, adjusted["sso"].ChangedAt)
	})

	t.Run("a license field restated at the value it held records nothing", func(t *testing.T) {
		w := newLicensedWorld(t)

		w.License().Adjust(ct.LicenseTemplateValues{"flows": 500})

		// This is a history of changes, not of requests.
		history := w.License().History()
		assert.Equal(t, int64(1), history.Total)
		assert.Equal(t, ct.INSTANTIATED, history.Items[0].Type)
	})

	t.Run("an empty adjustment records nothing", func(t *testing.T) {
		w := newLicensedWorld(t)

		w.License().Adjust(ct.LicenseTemplateValues{})

		assert.Equal(t, int64(1), w.License().History().Total)
	})

	t.Run("a refused adjustment records nothing", func(t *testing.T) {
		w := newLicensedWorld(t)

		resp := w.License().AdjustRaw(ct.LicenseTemplateValues{"flows": 100001})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))

		assert.Equal(t, int64(1), w.License().History().Total)
	})
}

func TestOrganizationLicenseHistoryRead(t *testing.T) {
	t.Run("reads newest first", func(t *testing.T) {
		w := newLicensedWorld(t)

		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 900})

		items := w.License().History().Items
		require.Len(t, items, 3)
		assert.InDelta(t, 900.0, items[0].NewValue, 0)
		assert.InDelta(t, 800.0, items[1].NewValue, 0)
		assert.Equal(t, ct.INSTANTIATED, items[2].Type)
	})

	t.Run("paginates without repeating or dropping an entry", func(t *testing.T) {
		w := newLicensedWorld(t)
		for _, value := range []int{600, 700, 800, 900} {
			w.License().Adjust(ct.LicenseTemplateValues{"flows": value})
		}

		firstPage := w.License().HistoryPage(historyPage(2, 0))
		secondPage := w.License().HistoryPage(historyPage(2, 2))

		assert.Equal(t, int64(5), firstPage.Total, "total counts every match, not the page")
		assert.Equal(t, 2, firstPage.Count)
		assert.Equal(t, int64(5), secondPage.Total)

		whole := w.License().History().Items
		require.Len(t, whole, 5)
		assert.Equal(t, entryIDs(whole[0:2]), entryIDs(firstPage.Items))
		assert.Equal(t, entryIDs(whole[2:4]), entryIDs(secondPage.Items))
	})

	t.Run("404 when the product has no organization with that identifier", func(t *testing.T) {
		w := newLicenseWorld(t)

		resp := w.License().For(missingOrganizationID()).HistoryRaw(
			ct.GetOrganizationLicenseHistoryParams{},
		)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("does not read another product's history", func(t *testing.T) {
		first := newLicensedWorld(t)
		second := newLicenseWorld(t)

		resp := second.License().For(first.OrganizationID()).HistoryRaw(
			ct.GetOrganizationLicenseHistoryParams{},
		)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("a write scope cannot read the history", func(t *testing.T) {
		w := newLicensedWorld(t)
		writeOnly, _ := w.product.CreateAPIKeyClientWithScopes(
			[]string{"organization_license:create", "organization_license:update"},
		)

		resp := w.License().As(writeOnly).HistoryRaw(ct.GetOrganizationLicenseHistoryParams{})
		assert.Equal(t, http.StatusForbidden, resp.StatusCode(), string(resp.Body))
	})
}

func TestOrganizationLicenseHistoryStorage(t *testing.T) {
	t.Run("entries carry no update timestamp", func(t *testing.T) {
		// Immutability is structural, not a rule the service remembers to keep:
		// there is no column to move and no update statement anywhere that
		// names this table.
		var count int
		err := testDB.QueryRow(
			`SELECT count(*) FROM information_schema.columns
			 WHERE table_name = 'organization_license_changes' AND column_name = 'updated_at'`,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "organization_license_changes has an updated_at column")
	})

	t.Run("deleting the organization takes its history with it", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})
		require.Equal(t, 2, historyRowCount(t, w.OrganizationID()))

		w.DeleteOrganization()

		assert.Equal(t, 0, historyRowCount(t, w.OrganizationID()))
	})

	t.Run("a license that predates the table gets an instantiation entry", func(t *testing.T) {
		w := newLicenseWorld(t)
		licenseID := ids.MustNew("lic")
		_, err := testDB.Exec(
			`INSERT INTO organization_licenses (
				id, platform_tenant_id, product_id, organization_id,
				template_id, values_json, instantiated_at
			) VALUES ($1, $2, $3, $4, $5, $6::jsonb, NOW())`,
			licenseID, w.tenantID, w.productID(), w.OrganizationID(), w.TemplateID(),
			`{"flows":500,"sso":true,"support_tier":"priority","region":"ca-central"}`,
		)
		require.NoError(t, err)

		_, err = testDB.Exec(backfillLicenseHistorySQL)
		require.NoError(t, err)

		history := w.License().History()
		require.Equal(t, int64(1), history.Total)
		require.NotNil(t, history.Items)
		require.Len(t, history.Items, 1)
		assert.Equal(t, ct.INSTANTIATED, history.Items[0].Type)
		assert.Equal(t, w.TemplateID(), deref(history.Items[0].TemplateId))
		assert.Equal(t, licenseID, history.Items[0].LicenseId)
	})
}

func historyRowCount(t *testing.T, organizationID string) int {
	t.Helper()
	var count int
	err := testDB.QueryRow(
		`SELECT count(*) FROM organization_license_changes WHERE organization_id = $1`,
		organizationID,
	).Scan(&count)
	require.NoError(t, err)
	return count
}

func historyPage(limit, offset int32) ct.GetOrganizationLicenseHistoryParams {
	return ct.GetOrganizationLicenseHistoryParams{Limit: &limit, Offset: &offset}
}

func entryIDs(entries []ct.OrganizationLicenseChangeResponse) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.Id)
	}
	return ids
}

// changesByField indexes adjustment entries by the license field they name.
func changesByField(
	entries []ct.OrganizationLicenseChangeResponse,
) map[string]ct.OrganizationLicenseChangeResponse {
	byField := make(map[string]ct.OrganizationLicenseChangeResponse, len(entries))
	for _, entry := range entries {
		if entry.Field != nil {
			byField[*entry.Field] = entry
		}
	}
	return byField
}

// newValueSet reads an instantiation entry's new value as a whole set of
// license field values. Only an instantiation records one; an adjustment
// records a single field's value.
func newValueSet(
	t *testing.T, entry ct.OrganizationLicenseChangeResponse,
) ct.LicenseTemplateValues {
	t.Helper()
	values, ok := entry.NewValue.(map[string]any)
	require.True(t, ok, "new_value is not a set of license field values")
	return ct.LicenseTemplateValues(values)
}

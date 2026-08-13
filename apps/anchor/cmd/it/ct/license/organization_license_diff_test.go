package license_ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrganizationLicenseDiff(t *testing.T) {
	t.Run("a fresh copy differs in nothing", func(t *testing.T) {
		w := newLicensedWorld(t)

		diff := w.License().Diff()

		assert.Equal(t, w.OrganizationID(), diff.OrganizationId)
		assert.Equal(t, w.TemplateID(), diff.TemplateId)
		assert.Empty(t, diff.Differences)
		assert.Equal(t, 0, diff.Count)
	})

	t.Run("an adjusted license field is reported with both sides", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})

		diff := w.License().Diff()

		// This is what an operator hunting bespoke accounts is looking for, and
		// both sides travel so the answer renders without a second read.
		require.Len(t, diff.Differences, 1)
		assert.Equal(t, "flows", diff.Differences[0].Field)
		assert.Equal(t, ct.Changed, diff.Differences[0].Kind)
		assert.InDelta(t, 800.0, diff.Differences[0].LicenseValue, 0)
		assert.InDelta(t, 500.0, diff.Differences[0].TemplateValue, 0)
		assert.Equal(t, 1, diff.Count)
	})

	t.Run("a template edited after instantiation shows as a difference", func(t *testing.T) {
		w := newLicensedWorld(t)

		// The organization is untouched here — the tier moved underneath it, which
		// the diff reports the same way it reports a deviation.
		w.Template().ReplaceValues(templateValuesWith("flows", 2000))

		diff := w.License().Diff()
		require.Len(t, diff.Differences, 1)
		assert.Equal(t, "flows", diff.Differences[0].Field)
		assert.Equal(t, ct.Changed, diff.Differences[0].Kind)
		assert.InDelta(t, 500.0, diff.Differences[0].LicenseValue, 0)
		assert.InDelta(t, 2000.0, diff.Differences[0].TemplateValue, 0)
	})

	t.Run("a license field the template gained after the copy", func(t *testing.T) {
		w := newLicensedWorld(t)
		widenSchema(t, w)

		difference := differenceByField(t, w.License().Diff().Differences, "seats")

		// The organization was never granted it, which is a different statement
		// from holding a different value for it.
		assert.Equal(t, ct.OnlyInTemplate, difference.Kind)
		assert.Nil(t, difference.LicenseValue)
		assert.InDelta(t, 25.0, difference.TemplateValue, 0)
	})

	t.Run("a license field the template dropped after the copy", func(t *testing.T) {
		w := newLicensedWorld(t)
		narrowSchema(t, w)

		difference := differenceByField(t, w.License().Diff().Differences, "region")

		assert.Equal(t, ct.OnlyInLicense, difference.Kind)
		assert.Equal(t, "ca-central", difference.LicenseValue)
		assert.Nil(t, difference.TemplateValue)
	})

	t.Run("differences are ordered by license field name", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{
			"flows": 800, "sso": false, "region": "eu-west",
		})

		diff := w.License().Diff()

		fields := make([]string, 0, len(diff.Differences))
		for _, difference := range diff.Differences {
			fields = append(fields, difference.Field)
		}
		assert.Equal(t, []string{"flows", "region", "sso"}, fields)
	})

	t.Run("404 when the organization has no license", func(t *testing.T) {
		w := newLicenseWorld(t)

		assert.Equal(t, http.StatusNotFound, w.License().DiffRaw().StatusCode())
	})

	t.Run("still answers after the template is archived", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})

		w.Template().Archive()

		// A withdrawn tier is still the tier this customer is on, so the question
		// "how does this account differ from what it was sold" keeps its answer
		// for the lifetime of the license.
		diff := w.License().Diff()
		assert.Equal(t, w.TemplateID(), diff.TemplateId)
		require.Len(t, diff.Differences, 1)
		assert.Equal(t, "flows", diff.Differences[0].Field)
	})

	t.Run("does not diff another product's license", func(t *testing.T) {
		first := newLicensedWorld(t)
		second := newLicenseWorld(t)

		resp := second.License().For(first.OrganizationID()).DiffRaw()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

// widenSchema declares one more license field and sets it on the template,
// leaving every already-instantiated organization without it.
func widenSchema(t *testing.T, w *licenseWorld) {
	t.Helper()
	w.RedeclareSchema(append(templateSchemaFields(), ct.LicenseFieldDeclaration{
		Name:       "seats",
		Type:       ct.LicenseFieldTypeLIMIT,
		Rules:      limitRules(0, 1000),
		UsageShape: new(ct.GAUGE),
	}))
	w.Template().ReplaceValues(templateValuesWith("seats", 25))
}

// narrowSchema drops one license field from the declaration and from the
// template, leaving every already-instantiated organization still holding it.
func narrowSchema(t *testing.T, w *licenseWorld) {
	t.Helper()
	fields := make([]ct.LicenseFieldDeclaration, 0, len(templateSchemaFields()))
	for _, declared := range templateSchemaFields() {
		if declared.Name != "region" {
			fields = append(fields, declared)
		}
	}
	w.RedeclareSchema(fields)
	w.Template().ReplaceValues(templateValuesExcept("region"))
}

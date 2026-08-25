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
		// Empty alone passes for both [] and null; this pins the JSON shape.
		assert.NotNil(t, diff.Differences)
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

	t.Run("a template edited after instantiation stops differing once followed", func(t *testing.T) {
		w := newLicensedWorld(t)

		// The organization is untouched here — the tier moved underneath it.
		// The license follows the edit, so once the propagation lands the
		// diff is clean again; only an adjusted field differs durably.
		w.Template().ReplaceValues(templateValuesWith("flows", 2000))

		waitForLicenseValues(t, w.License(), templateValuesWith("flows", 2000))
		diff := w.License().Diff()
		assert.Empty(t, diff.Differences)
		assert.Equal(t, 0, diff.Count)
	})

	t.Run("a license field the template gained lands on the license", func(t *testing.T) {
		w := newLicensedWorld(t)
		widenSchema(t, w)

		// The organization was licensed before `seats` was declared. The
		// template edit that sets it is propagated, so the gained field lands
		// rather than reading only_in_template forever.
		waitForLicenseValues(t, w.License(), templateValuesWith("seats", 25))
		assert.Empty(t, w.License().Diff().Differences)
	})

	t.Run("a license field the template dropped leaves the license", func(t *testing.T) {
		w := newLicensedWorld(t)
		narrowSchema(t, w)

		waitForLicenseValues(t, w.License(), templateValuesExcept("region"))
		assert.Empty(t, w.License().Diff().Differences)
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

// widenSchema declares one more license field and sets it on the template.
// The template edit is propagated, so already-instantiated organizations gain
// the field once the sync lands.
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

// narrowSchema drops one license field from the declaration. The removal
// cascades: the schema edit prunes the template, and the pruned template is
// propagated onto already-instantiated organizations. The explicit
// ReplaceValues restates what the cascade already wrote, so the template read
// is deterministic whichever runs first.
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

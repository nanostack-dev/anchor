package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// diffLicense reads the organization's license against the template it was
// instantiated from.
func diffLicense(t *testing.T, lc licenseCtx) *ct.GetOrganizationLicenseDiffResponse {
	t.Helper()
	resp, err := lc.product.OwnerAuthenticatedClient().GetOrganizationLicenseDiffWithResponse(
		context.Background(), lc.product.ProductID, lc.organizationID,
	)
	require.NoError(t, err)
	return resp
}

func TestGetOrganizationLicenseDiff(t *testing.T) {
	t.Run("a fresh copy differs in nothing", func(t *testing.T) {
		lc, template := licensedOrganization(t)

		resp := diffLicense(t, lc)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		assert.Equal(t, lc.organizationID, resp.JSON200.OrganizationId)
		assert.Equal(t, template.Id, resp.JSON200.TemplateId)
		assert.Empty(t, resp.JSON200.Differences)
		assert.Equal(t, 0, resp.JSON200.Count)
	})

	t.Run("an adjusted license field is reported with both sides", func(t *testing.T) {
		lc, _ := licensedOrganization(t)
		require.Equal(t, http.StatusOK, adjust(t, lc, ct.LicenseTemplateValues{"flows": 800}).StatusCode())

		resp := diffLicense(t, lc)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		// This is what an operator hunting bespoke accounts is looking for, and
		// both sides travel so the answer renders without a second read.
		require.Len(t, resp.JSON200.Differences, 1)
		difference := resp.JSON200.Differences[0]
		assert.Equal(t, "flows", difference.Field)
		assert.Equal(t, ct.Changed, difference.Kind)
		assert.InDelta(t, 800.0, difference.LicenseValue, 0)
		assert.InDelta(t, 500.0, difference.TemplateValue, 0)
		assert.Equal(t, 1, resp.JSON200.Count)
	})

	t.Run("a template edited after instantiation shows as a difference", func(t *testing.T) {
		lc, template := licensedOrganization(t)

		// The organization is untouched here — the tier moved underneath it.
		moved := ct.LicenseTemplateValues{
			"flows": 2000, "sso": true, "support_tier": "priority", "region": "ca-central",
		}
		update, err := lc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			lc.product.ProductID,
			template.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Values: &moved},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, update.StatusCode(), string(update.Body))

		resp := diffLicense(t, lc)
		require.NotNil(t, resp.JSON200)
		require.Len(t, resp.JSON200.Differences, 1)
		difference := resp.JSON200.Differences[0]
		assert.Equal(t, "flows", difference.Field)
		assert.Equal(t, ct.Changed, difference.Kind)
		assert.InDelta(t, 500.0, difference.LicenseValue, 0)
		assert.InDelta(t, 2000.0, difference.TemplateValue, 0)
	})

	t.Run("a license field the template gained after the copy", func(t *testing.T) {
		lc, template := licensedOrganization(t)
		widenSchema(t, lc, template.Id)

		resp := diffLicense(t, lc)
		require.NotNil(t, resp.JSON200)

		// The organization was never granted it, which is a different statement
		// from holding a different value for it.
		difference := differenceByField(t, resp.JSON200.Differences, "seats")
		assert.Equal(t, ct.OnlyInTemplate, difference.Kind)
		assert.Nil(t, difference.LicenseValue)
		assert.InDelta(t, 25.0, difference.TemplateValue, 0)
	})

	t.Run("a license field the template dropped after the copy", func(t *testing.T) {
		lc, template := licensedOrganization(t)
		narrowSchema(t, lc, template.Id)

		resp := diffLicense(t, lc)
		require.NotNil(t, resp.JSON200)

		difference := differenceByField(t, resp.JSON200.Differences, "region")
		assert.Equal(t, ct.OnlyInLicense, difference.Kind)
		assert.Equal(t, "ca-central", difference.LicenseValue)
		assert.Nil(t, difference.TemplateValue)
	})

	t.Run("differences are ordered by license field name", func(t *testing.T) {
		lc, _ := licensedOrganization(t)
		require.Equal(
			t, http.StatusOK,
			adjust(t, lc, ct.LicenseTemplateValues{
				"flows": 800, "sso": false, "region": "eu-west",
			}).StatusCode(),
		)

		resp := diffLicense(t, lc)
		require.NotNil(t, resp.JSON200)

		fields := make([]string, 0, len(resp.JSON200.Differences))
		for _, difference := range resp.JSON200.Differences {
			fields = append(fields, difference.Field)
		}
		assert.Equal(t, []string{"flows", "region", "sso"}, fields)
	})

	t.Run("404 when the organization has no license", func(t *testing.T) {
		lc := newOrganizationLicenseCtx(t)

		assert.Equal(t, http.StatusNotFound, diffLicense(t, lc).StatusCode())
	})

	t.Run("404 when the template has since been deleted", func(t *testing.T) {
		lc, template := licensedOrganization(t)

		del, err := lc.product.OwnerAuthenticatedClient().DeleteLicenseTemplateWithResponse(
			context.Background(), lc.product.ProductID, template.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, del.StatusCode(), string(del.Body))

		// The license itself is untouched — it stopped depending on the template
		// when the values were copied — but there is nothing left to compare
		// against.
		assert.Equal(t, http.StatusNotFound, diffLicense(t, lc).StatusCode())

		read, err := lc.product.OwnerAuthenticatedClient().GetOrganizationLicenseWithResponse(
			context.Background(), lc.product.ProductID, lc.organizationID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, read.StatusCode(), string(read.Body))
	})

	t.Run("does not diff another product's license", func(t *testing.T) {
		first, _ := licensedOrganization(t)
		second := newOrganizationLicenseCtx(t)

		resp, err := second.product.OwnerAuthenticatedClient().GetOrganizationLicenseDiffWithResponse(
			context.Background(), second.product.ProductID, first.organizationID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

// widenSchema declares one more license field and sets it on the template,
// leaving every already-instantiated organization without it.
func widenSchema(t *testing.T, lc licenseCtx, templateID string) {
	t.Helper()
	fields := append(templateSchemaFields(), ct.LicenseFieldDeclaration{
		Name:  "seats",
		Type:  ct.LicenseFieldTypeLIMIT,
		Rules: limitRules(0, 1000),
	})
	replaceSchemaFields(t, lc, fields)

	widened := validTemplateValues()
	widened["seats"] = 25
	replaceTemplateValues(t, lc, templateID, widened)
}

// narrowSchema drops one license field from the declaration and from the
// template, leaving every already-instantiated organization still holding it.
func narrowSchema(t *testing.T, lc licenseCtx, templateID string) {
	t.Helper()
	fields := make([]ct.LicenseFieldDeclaration, 0, len(templateSchemaFields()))
	for _, declared := range templateSchemaFields() {
		if declared.Name != "region" {
			fields = append(fields, declared)
		}
	}
	replaceSchemaFields(t, lc, fields)
	replaceTemplateValues(t, lc, templateID, templateValuesExcept("region"))
}

func replaceSchemaFields(t *testing.T, lc licenseCtx, fields []ct.LicenseFieldDeclaration) {
	t.Helper()
	resp, err := lc.product.OwnerAuthenticatedClient().UpdateLicenseSchemaWithResponse(
		context.Background(),
		lc.product.ProductID,
		ct.UpdateLicenseSchemaJSONRequestBody{Fields: &fields},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
}

func replaceTemplateValues(
	t *testing.T, lc licenseCtx, templateID string, values ct.LicenseTemplateValues,
) {
	t.Helper()
	resp, err := lc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
		context.Background(),
		lc.product.ProductID,
		templateID,
		ct.UpdateLicenseTemplateJSONRequestBody{Values: &values},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
}

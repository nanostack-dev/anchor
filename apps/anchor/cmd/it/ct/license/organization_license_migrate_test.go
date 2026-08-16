package license_ct_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// proValues is the tier the migration tests move onto: every declared license
// field set, and every one of them different from validTemplateValues, so a
// migration that only half-copied would fail rather than pass by coincidence.
func proValues() ct.LicenseTemplateValues {
	return ct.LicenseTemplateValues{
		"flows":        5000,
		"sso":          false,
		"support_tier": "basic",
		"region":       "eu-west",
	}
}

// schemaFieldsExcept drops one declaration, for the tests whose subject is a
// license field no template declares any more.
func schemaFieldsExcept(name string) []ct.LicenseFieldDeclaration {
	fields := templateSchemaFields()
	return slices.DeleteFunc(fields, func(f ct.LicenseFieldDeclaration) bool {
		return f.Name == name
	})
}

func migrateTo(templateID string, organizationIDs ...string) ct.OrganizationLicenseMigrationRequest {
	return ct.OrganizationLicenseMigrationRequest{
		TemplateId:      templateID,
		OrganizationIds: &organizationIDs,
	}
}

func TestMigrateOrganizationLicenses(t *testing.T) {
	t.Run("takes the target template's values and restamps the provenance", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())

		before := time.Now().Add(-time.Second)
		migration := w.Migration().Run(migrateTo(pro.Id, w.OrganizationID()))

		assert.Equal(t, pro.Id, migration.TemplateId)
		assert.Equal(t, 1, migration.Migrated)
		assert.Equal(t, ct.LicenseMigrationOutcomeMIGRATED, resultFor(t, migration, w.OrganizationID()).Outcome)

		// Provenance is the whole point: adjusting every field to Pro's values
		// would leave the license still saying it was sold the world's template.
		moved := w.License().Get()
		assertValues(t, moved.Values, proValues())
		assert.Equal(t, pro.Id, moved.TemplateId)
		assert.WithinRange(t, moved.InstantiatedAt, before, time.Now().Add(time.Second))
	})

	t.Run("reports what the organization moved from and what moved", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())

		migration := w.Migration().Run(migrateTo(pro.Id, w.OrganizationID()))

		result := resultFor(t, migration, w.OrganizationID())
		require.NotNil(t, result.PreviousTemplateId)
		assert.Equal(t, w.TemplateID(), *result.PreviousTemplateId)
		assert.Equal(t, len(result.Changes), result.Count)
		// Every declared field differs between the two tiers and this customer
		// holds no bespoke value, so every one moved.
		assert.Len(t, result.Changes, len(templateSchemaFields()))
		flows := differenceByField(t, result.Changes, "flows")
		assert.Equal(t, ct.Changed, flows.Kind)
		assert.InDelta(t, 500, flows.LicenseValue, 0)
		assert.InDelta(t, 5000, flows.TemplateValue, 0)
	})

	t.Run("leaves the license alone when it already holds the target", func(t *testing.T) {
		w := newLicensedWorld(t)
		instantiated := w.License().Get()

		migration := w.Migration().Run(migrateTo(w.TemplateID(), w.OrganizationID()))

		assert.Equal(t, 1, migration.Unchanged)
		assert.Equal(t, ct.LicenseMigrationOutcomeUNCHANGED, resultFor(t, migration, w.OrganizationID()).Outcome)

		// Nothing was written, so the date the copy was taken did not move and
		// the history gained no entry. That is what makes re-running a
		// migration safe.
		reread := w.License().Get()
		assert.Equal(t, instantiated.InstantiatedAt, reread.InstantiatedAt)
		assert.Len(t, w.License().History().Items, 1)
	})

	t.Run("moves every organization the source template still holds", func(t *testing.T) {
		w := newLicenseWorld(t)
		second := w.NewOrganization()
		w.License().Instantiate(w.TemplateID())
		w.License().For(second).Instantiate(w.TemplateID())
		pro := w.NewTemplate(proValues())

		migration := w.Migration().Run(ct.OrganizationLicenseMigrationRequest{
			TemplateId:     pro.Id,
			FromTemplateId: new(w.TemplateID()),
		})

		assert.Equal(t, 2, migration.Count)
		assert.Equal(t, 2, migration.Migrated)
		assertValues(t, w.License().Get().Values, proValues())
		assertValues(t, w.License().For(second).Get().Values, proValues())
	})

	t.Run("empties a withdrawn tier", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())
		w.Template().Archive()

		// Archived is refused as a target and accepted as a source: a tier
		// nobody can be sold is exactly the one customers have to be moved off.
		migration := w.Migration().Run(ct.OrganizationLicenseMigrationRequest{
			TemplateId:     pro.Id,
			FromTemplateId: new(w.TemplateID()),
		})

		assert.Equal(t, 1, migration.Migrated)
		assert.Equal(t, pro.Id, w.License().Get().TemplateId)
	})

	t.Run("records one history entry naming both tiers", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())

		w.Migration().Run(migrateTo(pro.Id, w.OrganizationID()))

		entry := w.License().History().Items[0]
		assert.Equal(t, ct.LicenseChangeTypeMIGRATED, entry.Type)
		require.NotNil(t, entry.TemplateId)
		assert.Equal(t, pro.Id, *entry.TemplateId)
		require.NotNil(t, entry.PreviousTemplateId)
		assert.Equal(t, w.TemplateID(), *entry.PreviousTemplateId)

		// The whole set on either side, not one entry per field: a tier change
		// is one event, and splitting it would read back as unrelated edits.
		assert.Equal(t, normaliseValues(validTemplateValues()), entry.OldValue)
		assert.Equal(t, normaliseValues(proValues()), entry.NewValue)
	})

	t.Run("every organization in one run shares a changed_at", func(t *testing.T) {
		w := newLicenseWorld(t)
		second := w.NewOrganization()
		w.License().Instantiate(w.TemplateID())
		w.License().For(second).Instantiate(w.TemplateID())
		pro := w.NewTemplate(proValues())

		migration := w.Migration().Run(migrateTo(pro.Id, w.OrganizationID(), second))

		// The run has no identifier of its own; the shared moment plus the
		// target template is what identifies it.
		first := w.License().History().Items[0]
		other := w.License().For(second).History().Items[0]
		assert.Equal(t, first.ChangedAt, other.ChangedAt)
		// The receipt reports the same instant, in whatever location the
		// response was decoded in — the entries come back from the database in
		// UTC, so this is an instant comparison rather than a value one.
		assert.WithinDuration(t, migration.MigratedAt, first.ChangedAt, 0)
	})
}

func TestMigrateOrganizationLicensesDifferences(t *testing.T) {
	t.Run("carries a bespoke value forward onto the new tier", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})
		pro := w.NewTemplate(proValues())

		migration := w.Migration().Run(migrateTo(pro.Id, w.OrganizationID()))

		assert.Equal(t, 1, migration.Migrated)
		// The tier moves, the deal survives: every field takes Pro's value
		// except the one this customer was given.
		assertValues(t, w.License().Get().Values, ct.LicenseTemplateValues{
			"flows":        800,
			"sso":          false,
			"support_tier": "basic",
			"region":       "eu-west",
		})
	})

	t.Run("a carried field is not reported as a change", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})
		pro := w.NewTemplate(proValues())

		migration := w.Migration().Run(migrateTo(pro.Id, w.OrganizationID()))

		// changes is what the move did, not how the two tiers differ. flows
		// stayed at 800, so it did not move.
		result := resultFor(t, migration, w.OrganizationID())
		assert.Equal(t, len(result.Changes), result.Count)
		for _, change := range result.Changes {
			assert.NotEqual(t, "flows", change.Field)
		}
		assert.Len(t, result.Changes, len(templateSchemaFields())-1)
	})

	t.Run("a template edited after the copy is carried forward too", func(t *testing.T) {
		w := newLicensedWorld(t)
		// Nobody adjusted this organization. Anchor cannot tell that apart from
		// a bespoke deal, so the stale value rides along — the cost the default
		// buys, and the reason to compare the two tiers first.
		w.Template().ReplaceValues(templateValuesWith("flows", 900))
		pro := w.NewTemplate(proValues())

		w.Migration().Run(migrateTo(pro.Id, w.OrganizationID()))

		assert.InDelta(t, 500, w.License().Get().Values["flows"], 0)
	})

	t.Run("discard takes the target template whole", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})
		pro := w.NewTemplate(proValues())

		request := migrateTo(pro.Id, w.OrganizationID())
		request.OnDifference = new(ct.DISCARD)
		migration := w.Migration().Run(request)

		assert.Equal(t, 1, migration.Migrated)
		assertValues(t, w.License().Get().Values, proValues())
	})

	t.Run("naming the tier it already holds resets it to that tier", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})

		// The out-of-scope re-sync, reachable only by naming the same template
		// and asking to discard. Nothing does this on its own.
		request := migrateTo(w.TemplateID(), w.OrganizationID())
		request.OnDifference = new(ct.DISCARD)
		migration := w.Migration().Run(request)

		assert.Equal(t, 1, migration.Migrated)
		assertValues(t, w.License().Get().Values, validTemplateValues())
	})

	t.Run("carrying forward onto the tier it already holds changes nothing", func(t *testing.T) {
		w := newLicensedWorld(t)
		w.License().Adjust(ct.LicenseTemplateValues{"flows": 800})

		migration := w.Migration().Run(migrateTo(w.TemplateID(), w.OrganizationID()))

		// Same tier, every difference kept: there is nothing left to write, so
		// the adjustment is not restamped over.
		assert.Equal(t, 1, migration.Unchanged)
		assert.InDelta(t, 800, w.License().Get().Values["flows"], 0)
	})

	t.Run("a field the target tier no longer declares is not carried forward", func(t *testing.T) {
		w := newLicensedWorld(t)
		// The schema drops a field, so no template declares it any more. The
		// license still holds it, and carrying it would resurrect a field
		// nothing validates.
		w.RedeclareSchema(schemaFieldsExcept("region"))
		pro := w.NewTemplate(templateValuesExcept("region"))

		w.Migration().Run(migrateTo(pro.Id, w.OrganizationID()))

		assert.NotContains(t, w.License().Get().Values, "region")
	})
}

func TestMigrateOrganizationLicensesIsolatesFailures(t *testing.T) {
	t.Run("skips an organization holding no license", func(t *testing.T) {
		w := newLicensedWorld(t)
		unlicensed := w.NewOrganization()
		pro := w.NewTemplate(proValues())

		migration := w.Migration().Run(migrateTo(pro.Id, w.OrganizationID(), unlicensed))

		assert.Equal(t, 1, migration.Migrated)
		assert.Equal(t, 1, migration.Skipped)
		result := resultFor(t, migration, unlicensed)
		require.NotNil(t, result.Reason)
		assert.Equal(t, ct.NOTLICENSED, *result.Reason)
		assert.Nil(t, result.PreviousTemplateId)
	})

	t.Run("one unknown organization does not abort the batch", func(t *testing.T) {
		w := newLicensedWorld(t)
		missing := missingOrganizationID()
		pro := w.NewTemplate(proValues())

		migration := w.Migration().Run(migrateTo(pro.Id, w.OrganizationID(), missing))

		assert.Equal(t, 1, migration.Migrated)
		assert.Equal(t, 1, migration.Failed)
		failed := resultFor(t, migration, missing)
		assert.Equal(t, ct.LicenseMigrationOutcomeFAILED, failed.Outcome)
		require.NotNil(t, failed.Error)
		assert.Equal(t, "ORGANIZATION_NOT_FOUND", failed.Error.Code)

		// A typo in the list is loud rather than silent, and the organizations
		// that were named correctly still moved.
		assertValues(t, w.License().Get().Values, proValues())
	})

	t.Run("another product's organization cannot be reached", func(t *testing.T) {
		first := newLicensedWorld(t)
		second := newLicensedWorld(t)
		pro := first.NewTemplate(proValues())

		migration := first.Migration().Run(migrateTo(pro.Id, second.OrganizationID()))

		assert.Equal(t, 1, migration.Failed)
		assertValues(t, second.License().Get().Values, validTemplateValues())
	})

	t.Run("an organization named twice is moved once", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())

		migration := w.Migration().Run(
			migrateTo(pro.Id, w.OrganizationID(), w.OrganizationID()),
		)

		assert.Equal(t, 1, migration.Count)
		assert.Len(t, w.License().History().Items, 2)
	})
}

func TestMigrateOrganizationLicensesRefusals(t *testing.T) {
	t.Run("400 when neither selection is supplied", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())

		resp := w.Migration().RunRaw(ct.OrganizationLicenseMigrationRequest{TemplateId: pro.Id})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertAPIError(t, resp.JSON400.Errors, "LICENSE_MIGRATION_SELECTION_INVALID")
	})

	t.Run("400 when both selections are supplied", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())

		request := migrateTo(pro.Id, w.OrganizationID())
		request.FromTemplateId = new(w.TemplateID())
		resp := w.Migration().RunRaw(request)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertAPIError(t, resp.JSON400.Errors, "LICENSE_MIGRATION_SELECTION_INVALID")
	})

	t.Run("400 when the target template is archived", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())
		w.ArchiveTemplateByID(pro.Id)

		resp := w.Migration().RunRaw(migrateTo(pro.Id, w.OrganizationID()))
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		assertAPIError(t, resp.JSON400.Errors, "LICENSE_TEMPLATE_ARCHIVED")
	})

	t.Run("404 when the target template does not exist", func(t *testing.T) {
		w := newLicensedWorld(t)

		resp := w.Migration().RunRaw(migrateTo(missingTemplateID(), w.OrganizationID()))
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})

	t.Run("404 when the source template does not exist", func(t *testing.T) {
		w := newLicensedWorld(t)
		pro := w.NewTemplate(proValues())

		// Not an empty successful run: a mistyped source would otherwise read
		// as "nobody was on that tier".
		resp := w.Migration().RunRaw(ct.OrganizationLicenseMigrationRequest{
			TemplateId:     pro.Id,
			FromTemplateId: new(missingTemplateID()),
		})
		require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
		assertAPIError(t, resp.JSON404.Errors, "LICENSE_MIGRATION_SOURCE_TEMPLATE_NOT_FOUND")
	})
}

package license

import "time"

// DifferencePolicy decides what a migration does with an Organization whose
// license differs from the template it currently holds.
//
// A difference is either a deviation or the template moving after the copy was
// taken, and the difference alone does not say which — see [DiffValues] — so
// DifferenceSkip protects a bespoke arrangement and a stale copy equally. The
// contract documents what each value means for a reader of the API. See
// LicenseMigrationDifferencePolicy in cmd/http/openapi.yaml.
type DifferencePolicy string

const (
	DifferenceSkip      DifferencePolicy = "SKIP"
	DifferenceOverwrite DifferencePolicy = "OVERWRITE"
)

// MigrationOutcome is what a migration did to one Organization.
//
// The contract documents what each one means for a reader of the API. See
// LicenseMigrationOutcome in cmd/http/openapi.yaml.
type MigrationOutcome string

const (
	// OutcomeMigrated is the Organization now holding the target template's
	// values, or — on a dry run — the Organization that would.
	OutcomeMigrated MigrationOutcome = "MIGRATED"
	// OutcomeUnchanged is the Organization already holding those values, from
	// that same template. Nothing was written and no history entry was
	// appended, which is what makes a second run of one migration a no-op.
	OutcomeUnchanged MigrationOutcome = "UNCHANGED"
	// OutcomeSkipped is the Organization deliberately left alone.
	// [OrganizationMigrationResult.Reason] says why.
	OutcomeSkipped MigrationOutcome = "SKIPPED"
	// OutcomeFailed is the write attempted and refused. The rest of the batch
	// is unaffected: each Organization is written in its own transaction.
	OutcomeFailed MigrationOutcome = "FAILED"
)

// MigrationSkipReason says why a migration left one Organization alone.
type MigrationSkipReason string

const (
	SkipDiffersFromTemplate MigrationSkipReason = "DIFFERS_FROM_TEMPLATE"
	SkipNotLicensed         MigrationSkipReason = "NOT_LICENSED"
)

// MaxMigrationOrganizations bounds one run. A selection matching more is
// refused, carrying its count, rather than truncated: a batch that silently
// covered part of what was asked for is worse than one that says it is too
// big. See docs/adr/0014-organization-licenses-are-migrated-in-bulk.md.
const MaxMigrationOrganizations = 500

// MigrateLicensesInput moves a set of Organizations onto one license template.
//
// Exactly one selection is supplied: OrganizationIDs names them, and
// FromTemplateID takes every Organization in the Product whose license names
// that template. The service refuses both and neither, because the two are
// alternatives rather than filters that compose.
type MigrateLicensesInput struct {
	TenantID        string `validate:"required,notblank"`
	ProductID       string `validate:"required,notblank"`
	TemplateID      string `validate:"required,notblank"`
	OrganizationIDs []string
	FromTemplateID  string
	OnDifference    DifferencePolicy
	DryRun          bool
}

// OrganizationMigrationResult is what one migration did to one Organization,
// and what its license fields looked like on either side.
type OrganizationMigrationResult struct {
	OrganizationID string
	Outcome        MigrationOutcome
	Reason         *MigrationSkipReason
	// PreviousTemplateID is the template the Organization held before the run.
	// Absent when it holds no license.
	PreviousTemplateID *string
	// Changes is how the Organization's license differed from the target
	// template before the run, ordered by license field name. LicenseValue is
	// what the Organization held, TemplateValue what the target grants.
	Changes []FieldDifference
	// Error is what refused the write. Set exactly when Outcome is
	// OutcomeFailed.
	Error error
}

// Migration is the receipt for one run.
type Migration struct {
	TemplateID string
	DryRun     bool
	// MigratedAt is the single clock the whole run shares: stamped as
	// InstantiatedAt on every Organization moved, and as ChangedAt on every
	// history entry appended. Together with TemplateID it is what identifies
	// the run, which is why no batch identifier is stored.
	MigratedAt time.Time
	Results    []OrganizationMigrationResult
}

// MigrationTally counts a run's results by outcome.
type MigrationTally struct {
	Migrated  int
	Unchanged int
	Skipped   int
	Failed    int
}

// Tally counts the run's results by outcome.
func (m Migration) Tally() MigrationTally {
	var tally MigrationTally
	for _, result := range m.Results {
		switch result.Outcome {
		case OutcomeMigrated:
			tally.Migrated++
		case OutcomeUnchanged:
			tally.Unchanged++
		case OutcomeSkipped:
			tally.Skipped++
		case OutcomeFailed:
			tally.Failed++
		}
	}
	return tally
}

// MigratedTo returns the Organization's license as the target template stamps
// it: the template's values copied whole, its identifier as provenance, and
// migratedAt as the moment the copy was taken. Identity, Product and
// Organization are kept.
//
// The values are copied rather than merged. A merge would have to decide which
// of the Organization's own values to carry forward, and nothing can tell a
// bespoke deviation from a template that moved.
func (l *OrganizationLicense) MigratedTo(
	target Template, migratedAt time.Time,
) OrganizationLicense {
	migrated := *l
	migrated.TemplateID = target.ID
	migrated.InstantiatedAt = migratedAt
	migrated.Values = target.Values
	return migrated
}

// NewMigrationChange records one Organization moving onto another template, as
// a single entry carrying the whole set on either side. An adjustment records
// one entry per field because a caller moves fields one at a time; a migration
// replaces the set, so splitting it per field would describe a tier change as a
// coincidence of unrelated edits.
func NewMigrationChange(
	migrated OrganizationLicense,
	previousTemplateID string,
	previousValues TemplateValues,
	changedAt time.Time,
) OrganizationLicenseChange {
	change := OrganizationLicenseChange{
		PlatformTenantID:   migrated.PlatformTenantID,
		ProductID:          migrated.ProductID,
		OrganizationID:     migrated.OrganizationID,
		LicenseID:          migrated.ID,
		Type:               ChangeMigrated,
		TemplateID:         new(migrated.TemplateID),
		PreviousTemplateID: new(previousTemplateID),
		OldValue:           map[string]any(previousValues),
		NewValue:           map[string]any(migrated.Values),
		ChangedAt:          changedAt,
	}
	change.GenerateID()
	return change
}

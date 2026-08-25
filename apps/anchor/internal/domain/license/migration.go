package license

import (
	"maps"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

// DifferencePolicy decides what a migration does with a license field whose
// value differs from the template the Organization currently holds.
//
// A difference is either a deviation or the template moving after the copy was
// taken, and the difference alone does not say which — see [DiffValues] — so
// CarryForwardDifferences preserves a stale copy as readily as a bespoke deal.
// The contract documents what each value means for a reader of the API. See
// LicenseMigrationDifferencePolicy in cmd/http/openapi.yaml.
type DifferencePolicy string

const (
	// CarryForwardDifferences keeps the Organization's own value on the
	// migrated license, so a bespoke arrangement survives a tier change. The
	// default.
	CarryForwardDifferences DifferencePolicy = "CARRY_FORWARD"
	// DiscardDifferences takes the target template whole.
	DiscardDifferences DifferencePolicy = "DISCARD"
)

// MigrationOutcome is what a migration did to one Organization.
//
// The contract documents what each one means for a reader of the API. See
// LicenseMigrationOutcome in cmd/http/openapi.yaml.
type MigrationOutcome string

const (
	// OutcomeChanged is the Organization's license written and its provenance
	// stamped onto the target — moved from another tier, or granted its first,
	// whichever the Organization held before the run.
	OutcomeChanged MigrationOutcome = "CHANGED"
	// OutcomeUnchanged is the Organization already holding exactly those
	// values, from that same template. Nothing was written and no history entry
	// was appended, which is what makes a second run of one migration a no-op.
	OutcomeUnchanged MigrationOutcome = "UNCHANGED"
	// OutcomeFailed is the write attempted and refused. The rest of the batch
	// is unaffected: each Organization is written in its own transaction.
	OutcomeFailed MigrationOutcome = "FAILED"
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
}

// OrganizationMigrationResult is what one migration did to one Organization,
// and what its license fields looked like on either side.
type OrganizationMigrationResult struct {
	OrganizationID string
	Outcome        MigrationOutcome
	// PreviousTemplateID is the template the Organization held before the run.
	// Absent when it holds no license.
	PreviousTemplateID *string
	// Changes is what the move did to the license, ordered by license field
	// name. LicenseValue is the value held before, TemplateValue the value held
	// after. A field carried forward does not appear, because it did not move.
	Changes []FieldDifference
	// Error is what refused the write. Set exactly when Outcome is
	// OutcomeFailed.
	Error error
}

// Migration is the receipt for one run.
type Migration struct {
	TemplateID string
	// MigratedAt is the single clock the whole run shares: stamped as
	// InstantiatedAt on every Organization moved, and as ChangedAt on every
	// history entry appended. Together with TemplateID it is what identifies
	// the run, which is why no batch identifier is stored.
	MigratedAt time.Time
	Results    []OrganizationMigrationResult
}

// MigrationTally counts a run's results by outcome.
type MigrationTally struct {
	Changed   int
	Unchanged int
	Failed    int
}

// Tally counts the run's results by outcome.
func (m Migration) Tally() MigrationTally {
	results := functional.Slice(m.Results)
	return MigrationTally{
		Changed:   results.CountBy(func(r OrganizationMigrationResult) bool { return r.Outcome == OutcomeChanged }),
		Unchanged: results.CountBy(func(r OrganizationMigrationResult) bool { return r.Outcome == OutcomeUnchanged }),
		Failed:    results.CountBy(func(r OrganizationMigrationResult) bool { return r.Outcome == OutcomeFailed }),
	}
}

// MigratedTo returns the Organization's license as the target template stamps
// it: the target's identifier as provenance, migratedAt as the moment the copy
// was taken, and the values MigratedValues resolves. Identity, Product and
// Organization are kept.
//
// AdjustedFields follows the policy: DiscardDifferences clears it,
// CarryForwardDifferences keeps the fields the target declares.
func (l *OrganizationLicense) MigratedTo(
	target Template, current TemplateValues, policy DifferencePolicy, migratedAt time.Time,
) OrganizationLicense {
	migrated := *l
	migrated.TemplateID = target.ID
	migrated.InstantiatedAt = migratedAt
	migrated.Values = MigratedValues(l.Values, current, target.Values, policy)
	migrated.AdjustedFields = migratedAdjustedFields(l.AdjustedFields, target.Values, policy)
	return migrated
}

func migratedAdjustedFields(
	adjusted []string, target TemplateValues, policy DifferencePolicy,
) []string {
	if policy == DiscardDifferences {
		return nil
	}
	var kept []string
	for _, name := range adjusted {
		if _, declared := target[name]; declared {
			kept = append(kept, name)
		}
	}
	return kept
}

// MigratedValues resolves what an Organization holds after a move onto target.
//
// Under DiscardDifferences that is the target, whole. Under the default a
// license field whose held value differs from current — the template the
// Organization is on today — keeps the held value instead, so a bespoke
// arrangement survives the move.
//
// Only license fields the target declares can be carried forward. A value the
// target does not name belongs to a field its schema no longer declares, and
// carrying it would resurrect a field nothing validates.
func MigratedValues(
	held, current, target TemplateValues, policy DifferencePolicy,
) TemplateValues {
	migrated := make(TemplateValues, len(target))
	maps.Copy(migrated, target)
	if policy == DiscardDifferences {
		return migrated
	}

	for name := range target {
		heldValue, isHeld := held[name]
		if !isHeld {
			continue
		}
		if currentValue, inCurrent := current[name]; !inCurrent || !valuesEqual(heldValue, currentValue) {
			migrated[name] = heldValue
		}
	}
	return migrated
}

// NewMigrationChange records one Organization's license set by the batch
// migrate route, as a single entry carrying the whole set on either side. An
// adjustment records one entry per field because a caller moves fields one at
// a time; this replaces the set, so splitting it per field would describe a
// tier change as a coincidence of unrelated edits.
//
// previousTemplateID and previousValues are nil when the Organization held no
// license before this run: it was granted one, not moved, and there is
// nothing to record on the old side.
func NewMigrationChange(
	migrated OrganizationLicense,
	previousTemplateID *string,
	previousValues TemplateValues,
	changedAt time.Time,
) OrganizationLicenseChange {
	change := OrganizationLicenseChange{
		PlatformTenantID:   migrated.PlatformTenantID,
		ProductID:          migrated.ProductID,
		OrganizationID:     migrated.OrganizationID,
		LicenseID:          migrated.ID,
		Type:               ChangeSet,
		TemplateID:         new(migrated.TemplateID),
		PreviousTemplateID: previousTemplateID,
		NewValue:           map[string]any(migrated.Values),
		ChangedAt:          changedAt,
	}
	if previousTemplateID != nil {
		change.OldValue = map[string]any(previousValues)
	}
	change.GenerateID()
	return change
}

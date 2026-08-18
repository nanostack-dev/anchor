package license

import (
	"maps"
	"slices"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// ChangeType names what happened to an Organization's license.
//
// The contract documents what each one means for a reader of the API. See
// LicenseChangeType in cmd/http/openapi.yaml.
type ChangeType string

const (
	// ChangeInstantiated is a template copied onto the Organization.
	// [OrganizationLicenseChange.TemplateID] names the template and NewValue
	// carries the whole set of values copied.
	ChangeInstantiated ChangeType = "INSTANTIATED"
	// ChangeAdjusted is one license field moved for this Organization alone.
	// [OrganizationLicenseChange.Field] names it, OldValue and NewValue are
	// that field's values on either side of the change.
	ChangeAdjusted ChangeType = "ADJUSTED"
	// ChangeSet is the Organization's license set to a template through the
	// batch migrate route — moved from another tier, or granted its first,
	// whichever the Organization held before the run.
	// [OrganizationLicenseChange.TemplateID] names the template it now holds,
	// PreviousTemplateID the one it came from (absent for a first license), and
	// OldValue and NewValue carry the whole set of values on either side
	// (OldValue absent to match).
	ChangeSet ChangeType = "SET"
)

// OrganizationLicenseChange is one entry in an Organization's license history.
//
// Append-only, following integration_audit_logs: the row carries no update
// timestamp and the repository exposes no update, so a correction is a later
// entry rather than an edit of this one.
type OrganizationLicenseChange struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	OrganizationID   string
	// LicenseID is which license record was changed. Provenance, not a live
	// dependency, exactly as [OrganizationLicense.TemplateID] is.
	LicenseID  string
	Type       ChangeType
	TemplateID *string
	// PreviousTemplateID is the template the Organization held before it was
	// moved. Set only by [NewMigrationChange].
	PreviousTemplateID *string
	Field              *string
	OldValue           any
	NewValue           any
	ChangedAt          time.Time
}

// NewInstantiationChange records one entry for the whole copied set.
func NewInstantiationChange(
	organizationLicense OrganizationLicense, changedAt time.Time,
) OrganizationLicenseChange {
	change := OrganizationLicenseChange{
		PlatformTenantID: organizationLicense.PlatformTenantID,
		ProductID:        organizationLicense.ProductID,
		OrganizationID:   organizationLicense.OrganizationID,
		LicenseID:        organizationLicense.ID,
		Type:             ChangeInstantiated,
		TemplateID:       new(organizationLicense.TemplateID),
		NewValue:         map[string]any(organizationLicense.Values),
		ChangedAt:        changedAt,
	}
	change.GenerateID()
	return change
}

// NewAdjustmentChanges records one entry per field that actually moved.
// Unchanged fields are omitted. Every entry shares changedAt.
func NewAdjustmentChanges(
	organizationLicense OrganizationLicense, previous TemplateValues, changedAt time.Time,
) []OrganizationLicenseChange {
	names := make(map[string]struct{}, len(previous)+len(organizationLicense.Values))
	for name := range previous {
		names[name] = struct{}{}
	}
	for name := range organizationLicense.Values {
		names[name] = struct{}{}
	}

	var changes []OrganizationLicenseChange
	for _, name := range slices.Sorted(maps.Keys(names)) {
		oldValue := previous[name]
		newValue := organizationLicense.Values[name]
		if valuesEqual(oldValue, newValue) {
			continue
		}
		change := OrganizationLicenseChange{
			PlatformTenantID: organizationLicense.PlatformTenantID,
			ProductID:        organizationLicense.ProductID,
			OrganizationID:   organizationLicense.OrganizationID,
			LicenseID:        organizationLicense.ID,
			Type:             ChangeAdjusted,
			Field:            new(name),
			OldValue:         oldValue,
			NewValue:         newValue,
			ChangedAt:        changedAt,
		}
		change.GenerateID()
		changes = append(changes, change)
	}
	return changes
}

// GenerateID sets the entry's ID to a new prefixed KSUID.
func (c *OrganizationLicenseChange) GenerateID() {
	c.ID = ids.MustNew("lchg")
}

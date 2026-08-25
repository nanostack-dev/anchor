package license

import (
	"maps"
	"slices"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// OrganizationLicense is one Organization's own copy of a [Template]'s values.
// An Organization has at most one.
//
// A copy that follows: the license holds its own values, and a later edit to
// the template's values is propagated onto them — except on AdjustedFields,
// where the Organization's bespoke value survives every template update. See
// docs/adr/0017-license-follows-its-template.md, which supersedes the
// copy-not-pointer rule of docs/adr/0004-license-schema-template-and-copy.md.
type OrganizationLicense struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	OrganizationID   string
	// TemplateID names the template this license was stamped from and now
	// follows. InstantiatedAt is when the copy was taken; a propagated
	// template update does not move it, only a migration restamps it.
	TemplateID     string
	InstantiatedAt time.Time
	Values         TemplateValues
	// AdjustedFields is every license field adjusted for this Organization,
	// ordered by name. A propagated template update leaves these fields
	// alone. Instantiation sets it empty, an adjustment adds every field it
	// moved, and a migration keeps it under CarryForwardDifferences (dropping
	// fields the target does not declare) and clears it under
	// DiscardDifferences.
	AdjustedFields []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// GenerateID sets the license's ID to a new prefixed KSUID.
func (l *OrganizationLicense) GenerateID() {
	l.ID = ids.MustNew("lic")
}

// OrganizationLicenseDiff is how one Organization's license differs from the
// template it was instantiated from, license field by license field.
type OrganizationLicenseDiff struct {
	OrganizationID string
	TemplateID     string
	Differences    []FieldDifference
}

// AdjustedValues returns Values with in merged over it. No key can be removed
// this way, which is the point — every license field its schema declares must
// stay set. See docs/adr/0009-every-license-field-is-mandatory.md.
func (l *OrganizationLicense) AdjustedValues(in TemplateValues) TemplateValues {
	adjusted := make(TemplateValues, len(l.Values)+len(in))
	maps.Copy(adjusted, l.Values)
	maps.Copy(adjusted, in)
	return adjusted
}

// RecordAdjustedFields adds the named license fields to AdjustedFields,
// keeping the set deduplicated and ordered by name.
func (l *OrganizationLicense) RecordAdjustedFields(fieldNames []string) {
	l.AdjustedFields = unionSorted(l.AdjustedFields, fieldNames)
}

func unionSorted(a, b []string) []string {
	set := make(map[string]struct{}, len(a)+len(b))
	for _, name := range a {
		set[name] = struct{}{}
	}
	for _, name := range b {
		set[name] = struct{}{}
	}
	return slices.Sorted(maps.Keys(set))
}

// SyncedValues resolves what the license holds after following its template:
// the template's values whole, except that each adjusted field the template
// still declares keeps the value held. An adjusted field the template no
// longer names is dropped with the field itself — carrying it would resurrect
// a field nothing validates, exactly as MigratedValues refuses to.
func (l *OrganizationLicense) SyncedValues(template TemplateValues) TemplateValues {
	synced := make(TemplateValues, len(template))
	maps.Copy(synced, template)
	for _, name := range l.AdjustedFields {
		heldValue, isHeld := l.Values[name]
		if !isHeld {
			continue
		}
		if _, declared := template[name]; declared {
			synced[name] = heldValue
		}
	}
	return synced
}

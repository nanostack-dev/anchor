package license

import (
	"maps"
	"slices"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// OrganizationLicense is one Organization's own copy of a [Template]'s values.
// An Organization has at most one. The copy follows its template except on
// AdjustedFields (docs/adr/0017-license-follows-its-template.md).
type OrganizationLicense struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	OrganizationID   string
	TemplateID       string
	InstantiatedAt   time.Time
	Values           TemplateValues
	AdjustedFields   []string
	CreatedAt        time.Time
	UpdatedAt        time.Time
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

package license

import (
	"maps"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// OrganizationLicense is one Organization's own copy of a [Template]'s values.
// An Organization has at most one.
//
// Copy, not pointer: editing a template afterwards cannot change a live
// customer, and per-organization deviation therefore needs no override layer.
// See docs/adr/0004-license-schema-template-and-copy.md.
type OrganizationLicense struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	OrganizationID   string
	// TemplateID and InstantiatedAt are provenance, not a live dependency. The
	// template they name can be edited or deleted without touching Values.
	TemplateID     string
	InstantiatedAt time.Time
	Values         TemplateValues
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

package license

import (
	"maps"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// OrganizationLicense is one Organization's own copy of a [Template]'s values.
// An Organization has at most one.
//
// Copy, not pointer. Editing a template afterwards cannot change a live
// customer, and this record is the historical statement of what that customer
// was sold. It is also why per-organization deviation needs no override layer:
// a bespoke limit for one customer is an edit to this record's values. See
// docs/adr/0004-license-schema-template-and-copy.md.
type OrganizationLicense struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	OrganizationID   string
	// TemplateID and InstantiatedAt are provenance: which template this
	// Organization was stamped from, and when. They answer "what were they
	// sold", and they are what the diff against the current template is
	// computed from. Neither is a live dependency — the template can be edited
	// or deleted without touching Values.
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

// AdjustedValues returns Values with in merged over it: a key present in in
// replaces the one held, and a key absent from in is left alone.
//
// Merge rather than replace, because a license is adjusted one field at a time
// for one customer, unlike a template which is authored whole. A wholesale
// replace would make a stale client silently revert a deviation it never read.
// No key can be removed this way, which is the point — every license field its
// schema declares must stay set. See
// docs/adr/0009-every-license-field-is-mandatory.md.
func (l *OrganizationLicense) AdjustedValues(in TemplateValues) TemplateValues {
	adjusted := make(TemplateValues, len(l.Values)+len(in))
	maps.Copy(adjusted, l.Values)
	maps.Copy(adjusted, in)
	return adjusted
}

package license

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// Template is a named set of values satisfying a Product's [Schema]: "Free" and
// "Pro" as reusable objects rather than values retyped for every customer.
//
// A template is mutable and unversioned. There is no draft/published/archived
// lifecycle and no version number, because a template is consulted once — at
// instantiation, when its values are copied onto an Organization's license.
// An email template resolves at send time and is therefore a live dependency;
// this one is a stamp. See docs/adr/0004-license-schema-template-and-copy.md.
type Template struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	Name             string
	Description      string
	Values           TemplateValues
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// GenerateID sets the template's ID to a new prefixed KSUID.
func (t *Template) GenerateID() {
	t.ID = ids.MustNew("ltpl")
}

// TemplateValues is what a template sets, keyed by license field name. A value
// is whatever its declared [FieldType] admits — a number, a boolean, or a
// string — so the map is deliberately untyped and every value is checked
// against its field's declaration before it is stored.
//
// This is a defined type rather than a bare map so the API contract can map
// onto it, keeping the generated package free of a hand-written conversion.
type TemplateValues map[string]any

package license

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// Template is a named set of values satisfying a Product's [Schema]: "Free" and
// "Pro" as reusable objects rather than values retyped for every customer.
//
// A template is mutable and unversioned. Its values are copied at
// instantiation and followed thereafter, except on adjusted fields
// (docs/adr/0017-license-follows-its-template.md).
//
// It does carry one lifecycle step, [TemplateStatus]: withdrawing a tier
// archives it and never deletes the row, because a license names the template
// it came from. See docs/adr/0010-license-templates-are-archived.md.
type Template struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	Name             string
	Description      string
	Status           TemplateStatus
	Values           TemplateValues
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// TemplateStatus is whether a template is still on sale.
//
// The two states are not a workflow. There is no draft to publish and no route
// back from archived: a tier is offered or it is withdrawn, and withdrawing it
// is what an operator means by deleting it.
type TemplateStatus string

const (
	// TemplateActive is on sale. It can be instantiated, edited and listed.
	TemplateActive TemplateStatus = "ACTIVE"
	// TemplateArchived is withdrawn. It stays readable by identifier so the
	// licenses that name it keep resolving, and it is refused everywhere a
	// caller would act as though the tier were still offered.
	TemplateArchived TemplateStatus = "ARCHIVED"
)

// IsArchived reports a withdrawn tier.
func (t *Template) IsArchived() bool {
	return t.Status == TemplateArchived
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

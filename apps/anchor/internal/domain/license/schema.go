// Package license holds the licensing domain types: what a Product declares its
// licenses may contain, and later what a license carries and what has been used
// against it.
//
// Anchor validates but never gates. Nothing in this package blocks an action;
// it describes and constrains writes only.
package license

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// Schema is a Product's declaration of every field its licenses may carry.
// Exactly one exists per Product — a second would have no meaning, since the
// schema is defined as the per-Product statement of what a license can hold.
type Schema struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	Description      string
	Fields           []Field
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// GenerateID sets the schema's ID to a new prefixed KSUID.
func (s *Schema) GenerateID() {
	s.ID = ids.MustNew("lsch")
}

// FieldByName returns the declared field with the given name, or nil. The usage
// path uses it to answer "is this reported key declared, and is it a limit"
// without loading the whole schema into a map at every call site.
func (s *Schema) FieldByName(name string) *Field {
	return functional.Slice(s.Fields).
		FindFirst(func(f Field) bool { return f.Name == name }).
		ToPtr()
}

// Field is one declared license field: its name, its type, and the rules a
// value must satisfy.
//
// There is no optionality flag. Every license template must set every field its
// schema declares, so a reader of a template never has to invent what an absent
// field means. See docs/adr/0009-every-license-field-is-mandatory.md.
//
// Rules constrain decisions, not observations. They bound what a limit may be
// set to and are never applied to a reported usage value — doing so would make
// the "exceeded" status unreachable.
//
// UsageShape is set exactly on a Limit field, and nil everywhere else: it
// names the one question Rules never answers, which is not "what may this be
// set to" but "what does a reported usage value against it look like." See
// docs/adr/0013-usage-shape-is-declared-not-inferred.md.
type Field struct {
	ID          string
	SchemaID    string
	Name        string
	Type        FieldType
	Description string
	Rules       FieldRules
	UsageShape  *UsageShape
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// GenerateID sets the field's ID to a new prefixed KSUID.
func (f *Field) GenerateID() {
	f.ID = ids.MustNew("lfld")
}

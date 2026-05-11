package email

import (
	"encoding/json"
	"time"

	"github.com/nanostack-dev/shared/toolkit"
)

// VariableType describes the expected runtime type for a declared template
// variable. Used for render-time validation so missing or wrong-typed values
// fail before SMTP submission.
type VariableType string

const (
	VariableTypeString VariableType = "STRING"
	VariableTypeNumber VariableType = "NUMBER"
	VariableTypeBool   VariableType = "BOOL"
	VariableTypeList   VariableType = "LIST"
	VariableTypeObject VariableType = "OBJECT"
)

// VariableSchemaProperty is a named field within an OBJECT variable or
// within the element schema of a LIST variable.
type VariableSchemaProperty struct {
	Name string       `json:"name"`
	Type VariableType `json:"type"`
}

// VariableSchemaItems describes the element shape for LIST variables.
type VariableSchemaItems struct {
	Type       VariableType             `json:"type"`
	Properties []VariableSchemaProperty `json:"properties,omitempty"`
}

// VariableSchema is the per-variable contract attached to a template version.
// The UI uses it to render a structured "test send" form; the renderer uses
// it to provide best-effort placeholder rendering for missing values.
type VariableSchema struct {
	Name       string                   `json:"name"`
	Type       VariableType             `json:"type"`
	Required   bool                     `json:"required"`
	Help       string                   `json:"help,omitempty"`
	Items      *VariableSchemaItems     `json:"items,omitempty"`
	Properties []VariableSchemaProperty `json:"properties,omitempty"`
}

// TemplateVersionStatus is the publication state of a single template version.
//
// Lifecycle:
//   - DRAFT: editable; visible only to authoring tools and TestSend.
//   - PUBLISHED: immutable; the version returned to API senders by default.
//   - ARCHIVED: previously-published version superseded by a newer publish.
type TemplateVersionStatus string

const (
	TemplateVersionStatusDraft     TemplateVersionStatus = "DRAFT"
	TemplateVersionStatusPublished TemplateVersionStatus = "PUBLISHED"
	TemplateVersionStatusArchived  TemplateVersionStatus = "ARCHIVED"
)

// TemplateExample is a named set of variable values used for preview and test
// sends in the template builder UI.
type TemplateExample struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Variables map[string]any `json:"variables"`
}

// Template is the per-product email template envelope. It carries identity
// (slug, name) and pointers to the current draft + published versions.
//
// Authoring content (subject, bodies, variables) lives on TemplateVersion so
// publishing a draft is an atomic pointer swap that never mutates an existing
// PUBLISHED row — guaranteeing senders never observe a half-edited template.
type Template struct {
	ID                 string
	PlatformTenantID   string
	ProductID          string
	Slug               string
	Name               string
	Description        string
	DraftVersionID     *string
	PublishedVersionID *string
	IsActive           bool
	Examples           []TemplateExample
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// GenerateID sets the template's ID to a new prefixed KSUID.
func (t *Template) GenerateID() {
	t.ID = toolkit.NewID("etpl")
}

// TemplateVersion is the immutable-once-published snapshot of a template's
// authoring fields. Drafts are mutable in place; publishing re-stamps the
// current draft as PUBLISHED, archives the previously-published version, and
// creates a fresh DRAFT cloned from the now-published content for further
// edits.
type TemplateVersion struct {
	ID            string
	TemplateID    string
	VersionNumber int32
	Subject       string
	BodyHTML      string
	BodyText      string
	Variables     []VariableSchema
	Status        TemplateVersionStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
	PublishedAt   *time.Time
}

// GenerateID sets the version's ID to a new prefixed KSUID.
func (v *TemplateVersion) GenerateID() {
	v.ID = toolkit.NewID("etplv")
}

// VariablesJSON serialises the variable schema for storage in JSONB.
func (v *TemplateVersion) VariablesJSON() (json.RawMessage, error) {
	if len(v.Variables) == 0 {
		return json.RawMessage("[]"), nil
	}
	return json.Marshal(v.Variables)
}

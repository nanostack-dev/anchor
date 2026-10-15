package email

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/searchx"
)

// CreateTemplateInput is the request shape for authoring a new template.
//
// On create the service writes the envelope row plus a single DRAFT version
// holding the subject/body/variables. The template starts unpublished;
// callers must invoke Publish to make it sendable via the public API.
type CreateTemplateInput struct {
	TenantID    string `validate:"required,notblank"`
	ProductID   string `validate:"required,notblank"`
	Slug        string `validate:"required,notblank"`
	Name        string `validate:"required,notblank"`
	Description string
	Subject     string `validate:"required,notblank"`
	BodyHTML    string `validate:"required,notblank"`
	BodyText    string
	Variables   []VariableSchema
}

// UpdateTemplateInput edits the template envelope (name, description, active
// flag). Authoring content (subject, bodies, variables) is edited via the
// draft version using UpdateTemplateDraftInput so PUBLISHED rows stay
// immutable.
type UpdateTemplateInput struct {
	TenantID    string `validate:"required"`
	ProductID   string `validate:"required"`
	ID          string `validate:"required"`
	Name        *string
	Description *string
	IsActive    *bool
}

// UpdateTemplateDraftInput edits the authoring content of the template's
// current DRAFT version. If no draft exists (e.g. the template was just
// published and no new draft was opened yet) the service creates one cloned
// from the published version.
type UpdateTemplateDraftInput struct {
	TenantID   string `validate:"required"`
	ProductID  string `validate:"required"`
	TemplateID string `validate:"required"`
	Subject    *string
	BodyHTML   *string
	BodyText   *string
	Variables  *[]VariableSchema
}

// PublishTemplateInput promotes the template's current DRAFT to PUBLISHED.
// The previously-published version (if any) is moved to ARCHIVED. After a
// successful publish a fresh DRAFT is opened cloned from the just-published
// content, so subsequent edits do not modify the live version.
type PublishTemplateInput struct {
	TenantID   string `validate:"required"`
	ProductID  string `validate:"required"`
	TemplateID string `validate:"required"`
}

// GetTemplateDraftInput is the request shape for fetching (or lazily creating)
// the current DRAFT version of a template. If no draft exists and a published
// version is present, a new draft is cloned from the published content.
type GetTemplateDraftInput struct {
	TenantID   string `validate:"required"`
	ProductID  string `validate:"required"`
	TemplateID string `validate:"required"`
}

// GetTemplateInput is the request shape for fetching a single template by ID.
type GetTemplateInput struct {
	TenantID  string `validate:"required"`
	ProductID string `validate:"required"`
	ID        string `validate:"required"`
}

// SortFieldTemplate names a column a template search may order by.
type SortFieldTemplate string

const (
	SortFieldTemplateCreatedAt SortFieldTemplate = "created_at"
	SortFieldTemplateUpdatedAt SortFieldTemplate = "updated_at"
	SortFieldTemplateName      SortFieldTemplate = "name"
	SortFieldTemplateSlug      SortFieldTemplate = "slug"
)

// SearchTemplatesFilter narrows a template search. Empty fields do not filter.
type SearchTemplatesFilter struct {
	IDs      []string
	Slugs    []string
	IsActive *bool
}

// SearchTemplatesInput is the request shape for searching a product's templates.
type SearchTemplatesInput struct {
	TenantID  string                                                   `validate:"required"`
	ProductID string                                                   `validate:"required"`
	Request   search.Request[SearchTemplatesFilter, SortFieldTemplate] `validate:"required"`
}

// Validate checks the input and the sort the client named.
func (in SearchTemplatesInput) Validate() error {
	return searchx.ValidateRequest(in, in.Request,
		SortFieldTemplateCreatedAt,
		SortFieldTemplateUpdatedAt,
		SortFieldTemplateName,
		SortFieldTemplateSlug)
}

// DeleteTemplateInput is the request shape for removing a template (and all
// its versions cascade-deleted).
type DeleteTemplateInput struct {
	TenantID  string `validate:"required"`
	ProductID string `validate:"required"`
	ID        string `validate:"required"`
}

// SendInput is the request shape for dispatching a transactional email.
//
// Either TemplateID or TemplateSlug must be supplied (slug is the friendlier
// option from product code; ID is the safer option from admin tools).
//
// By default the send uses the template's PUBLISHED version. Set UseDraft to
// dispatch the current DRAFT instead — reserved for trusted admin/preview
// flows; the public product-facing API always sends the published version.
//
// DedupeKey is the strongest defense against bad loops: when set, repeated
// submissions with the same key for the same product return the existing
// record instead of resending. Highly recommended.
type SendInput struct {
	TenantID     string `validate:"required"`
	ProductID    string `validate:"required"`
	TemplateID   *string
	TemplateSlug *string
	ToAddress    string `validate:"required,email"`
	ToName       string
	Variables    map[string]any
	DedupeKey    *string
	UseDraft     bool
	IsTestSend   bool
}

// TestSendInput is the request shape for the UI's "send test" button. It
// renders the template's current DRAFT version with the provided variables
// and dispatches once, flagged so it does not consume the product's
// per-template metrics.
type TestSendInput struct {
	TenantID   string `validate:"required"`
	ProductID  string `validate:"required"`
	TemplateID string `validate:"required"`
	ToAddress  string `validate:"required,email"`
	Variables  map[string]any
}

// PreviewInput is the request shape for the UI's render-preview pane (no
// SMTP submission, just the rendered HTML/text/subject). Previews always
// render the current DRAFT version unless UsePublished is set.
type PreviewInput struct {
	TenantID     string `validate:"required"`
	ProductID    string `validate:"required"`
	TemplateID   string `validate:"required"`
	Variables    map[string]any
	UsePublished bool
}

// PreviewResult is the rendered output returned by the preview endpoint.
type PreviewResult struct {
	Subject  string
	BodyHTML string
	BodyText string
	// Warnings lists non-fatal render issues (unknown vars, missing values, etc.).
	Warnings []string
}

// SaveTemplateExamplesInput replaces the full list of named variable example
// sets on the template envelope row.
type SaveTemplateExamplesInput struct {
	TenantID   string `validate:"required"`
	ProductID  string `validate:"required"`
	TemplateID string `validate:"required"`
	Examples   []TemplateExample
}

// SortFieldSend names a column a send-record search may order by.
type SortFieldSend string

const (
	SortFieldSendCreatedAt SortFieldSend = "created_at"
	SortFieldSendStatus    SortFieldSend = "status"
)

// SearchSendsFilter narrows a send-record search. Empty fields do not filter.
type SearchSendsFilter struct {
	TemplateIDs []string
	Statuses    []SendStatus `validate:"omitempty,dive,oneof=QUEUED SENDING SENT FAILED"`
}

// SearchSendsInput is the request shape for searching the send history.
type SearchSendsInput struct {
	TenantID  string                                           `validate:"required"`
	ProductID string                                           `validate:"required"`
	Request   search.Request[SearchSendsFilter, SortFieldSend] `validate:"required"`
}

// Validate checks the input and the sort the client named.
func (in SearchSendsInput) Validate() error {
	return searchx.ValidateRequest(in, in.Request,
		SortFieldSendCreatedAt,
		SortFieldSendStatus)
}

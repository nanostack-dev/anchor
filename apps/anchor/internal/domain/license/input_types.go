package license

// FieldDeclaration is one license field as supplied by a caller: the authored
// shape, before an ID or timestamps exist.
type FieldDeclaration struct {
	Name        string `validate:"required,notblank,max=120"`
	Type        FieldType
	Description string
	Rules       FieldRules
}

// CreateSchemaInput declares a Product's license schema for the first time.
//
// Fields may be empty: a Product is allowed to declare a schema up front and
// fill it in later, and an empty declaration is not malformed.
type CreateSchemaInput struct {
	TenantID    string `validate:"required,notblank"`
	ProductID   string `validate:"required,notblank"`
	Description string
	Fields      []FieldDeclaration `validate:"dive"`
}

// UpdateSchemaInput edits an existing schema. A nil member is left untouched;
// a non-nil Fields replaces the declaration wholesale, because a schema is a
// single statement and a partial edit has no unambiguous meaning for removals.
type UpdateSchemaInput struct {
	TenantID    string `validate:"required,notblank"`
	ProductID   string `validate:"required,notblank"`
	Description *string
	Fields      *[]FieldDeclaration `validate:"omitempty,dive"`
}

// GetSchemaInput is the request shape for reading a Product's license schema.
type GetSchemaInput struct {
	TenantID  string `validate:"required,notblank"`
	ProductID string `validate:"required,notblank"`
}

// DeleteSchemaInput is the request shape for removing a Product's license
// schema. Its fields cascade with it.
type DeleteSchemaInput struct {
	TenantID  string `validate:"required,notblank"`
	ProductID string `validate:"required,notblank"`
}

// CreateTemplateInput defines a new license template for a Product.
//
// Values may be empty only where the schema declares nothing required; the
// service decides that, because it is a property of the schema rather than of
// the request.
type CreateTemplateInput struct {
	TenantID    string `validate:"required,notblank"`
	ProductID   string `validate:"required,notblank"`
	Name        string `validate:"required,notblank,max=120"`
	Description string
	Values      TemplateValues
}

// UpdateTemplateInput edits an existing template. A nil member is left
// untouched; a non-nil Values replaces the set wholesale, because a template is
// one statement of what a tier grants and a partial edit has no unambiguous
// meaning for a field the caller dropped.
type UpdateTemplateInput struct {
	TenantID    string  `validate:"required,notblank"`
	ProductID   string  `validate:"required,notblank"`
	TemplateID  string  `validate:"required,notblank"`
	Name        *string `validate:"omitempty,notblank,max=120"`
	Description *string
	Values      *TemplateValues
}

// GetTemplateInput is the request shape for reading one license template.
type GetTemplateInput struct {
	TenantID   string `validate:"required,notblank"`
	ProductID  string `validate:"required,notblank"`
	TemplateID string `validate:"required,notblank"`
}

// ListTemplatesInput is the request shape for listing a Product's license
// templates. Every template the Product has ever offered is listed, because a
// template is never deleted. A non-nil Status narrows to one of them.
type ListTemplatesInput struct {
	TenantID  string `validate:"required,notblank"`
	ProductID string `validate:"required,notblank"`
	Status    *TemplateStatus
}

// ArchiveTemplateInput is the request shape for withdrawing one license
// template. The row is kept, because the Organizations licensed from it name it
// as the statement of what they were sold. See
// docs/adr/0010-license-templates-are-archived.md.
type ArchiveTemplateInput struct {
	TenantID   string `validate:"required,notblank"`
	ProductID  string `validate:"required,notblank"`
	TemplateID string `validate:"required,notblank"`
}

// DeleteTemplateInput is the request shape for removing a license template
// outright. Refused if any Organization license names it: use
// ArchiveTemplateInput once a template might have customers. See
// docs/adr/0011-unreferenced-license-template-can-be-deleted.md.
type DeleteTemplateInput struct {
	TenantID   string `validate:"required,notblank"`
	ProductID  string `validate:"required,notblank"`
	TemplateID string `validate:"required,notblank"`
}

// InstantiateLicenseInput copies a template's values onto an Organization. It is
// refused when the Organization already holds a license.
type InstantiateLicenseInput struct {
	TenantID       string `validate:"required,notblank"`
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	TemplateID     string `validate:"required,notblank"`
}

// GetLicenseInput is the request shape for reading an Organization's license,
// and for diffing it against the template it was instantiated from.
type GetLicenseInput struct {
	TenantID       string `validate:"required,notblank"`
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
}

// AdjustLicenseInput adjusts one Organization's license without touching the
// template. Values are merged, not replaced — the opposite of
// [UpdateTemplateInput]. See OrganizationLicenseAdjustRequest in the contract.
type AdjustLicenseInput struct {
	TenantID       string `validate:"required,notblank"`
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	Values         TemplateValues
}

package license

// FieldDeclaration is one license field as supplied by a caller: the authored
// shape, before an ID or timestamps exist.
type FieldDeclaration struct {
	Name        string `validate:"required,notblank,max=120"`
	Type        FieldType
	Required    bool
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

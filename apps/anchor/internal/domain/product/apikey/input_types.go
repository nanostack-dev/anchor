package apikey

// CreateProductAPIKeyInput represents input for creating a product API key.
type CreateProductAPIKeyInput struct {
	ProductID   string  `validate:"required,notblank"`
	Name        string  `validate:"required,notblank,max=100"`
	Description *string `validate:"omitempty,max=500"`
	Mutable     bool
	Permissions []string `validate:"required,dive,notblank,min=1"`
}

type GetProductAPIKeyInput struct {
	ProductID string `validate:"required,notblank"`
	ID        string `validate:"required,notblank"`
}

type UpdateProductAPIKeyInput struct {
	ProductID   string    `validate:"required,notblank"`
	ID          string    `validate:"required,notblank"`
	Name        *string   `validate:"required,notblank,max=100"`
	Description *string   `validate:"omitempty,max=500"`
	Permissions *[]string `validate:"omitempty,dive,notblank"`
}

type DeleteProductAPIKeyInput struct {
	ProductID string `validate:"required"`
	ID        string `validate:"required"`
}

// SortFieldProductAPIKey is the sort_by type of POST /api-keys/search. The
// generated search owns the values; this type gives the OpenAPI schema its Go type.
type SortFieldProductAPIKey string

type ValidateAPIKeyScopesInput struct {
	ProductID   string   `validate:"required,notblank"`
	Scopes      []string `validate:"required,dive,notblank"`
	APIKeyValue string   `validate:"required,notblank"`
}

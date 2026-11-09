package product

type CreateProductInput struct {
	TenantID    string `json:"tenant_id"   validate:"required,notblank"`
	Name        string `json:"name"        validate:"required,notblank,min=2,max=100"`
	Description string `json:"description" validate:"omitempty,max=1000"`
	Config      Config `json:"config"`
}

type UpdateProductInput struct {
	TenantID    string             `json:"tenant_id"             validate:"required,notblank"`
	ProductID   string             `json:"product_id"            validate:"required,notblank"`
	Name        *string            `json:"name,omitempty"        validate:"omitempty,notblank,min=2,max=100"`
	Description *string            `json:"description,omitempty" validate:"omitempty,max=1000"`
	Config      *UpdateConfigInput `json:"config,omitempty"`
}

type UpdateConfigInput struct {
	Protected           *bool `json:"protected,omitempty"`
	OrganizationAPIKeys OrganizationAPIKeysConfig
	Events              *EventsConfig
}

// SortFieldProduct is the sort_by type of POST /products/search. The generated
// search owns the values; this type gives the OpenAPI schema its Go type.
type SortFieldProduct string

type GetProductInput struct {
	TenantID  string `json:"tenant_id"  validate:"required,notblank"`
	ProductID string `json:"product_id" validate:"required,notblank"`
}

type DeleteProductInput struct {
	TenantID  string `json:"tenant_id"  validate:"required,notblank"`
	ProductID string `json:"product_id" validate:"required,notblank"`
}

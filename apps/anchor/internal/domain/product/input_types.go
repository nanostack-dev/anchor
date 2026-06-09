package product

import "github.com/nanostack-dev/nanostack-framework/pkg/search"

type CreateProductInput struct {
	TenantID    string `json:"tenant_id"   validate:"required,notblank"`
	Name        string `json:"name"        validate:"required,notblank"`
	Description string `json:"description" validate:"omitempty,max=1000"`
	Config      Config `json:"config"`
}

type UpdateProductInput struct {
	TenantID    string  `json:"tenant_id"             validate:"required,notblank"`
	ProductID   string  `json:"product_id"            validate:"required,notblank"`
	Name        *string `json:"name,omitempty"        validate:"omitempty,notblank"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=1000"`
	Config      *Config `json:"config,omitempty"`
}

type SearchProductFilter struct {
	IDs   []string `json:"ids,omitempty"   validate:"omitempty,dive,notblank"`
	Names []string `json:"names,omitempty" validate:"omitempty,dive,notblank"`
}

type SortFieldProduct string

const (
	SortFieldProductCreatedAt SortFieldProduct = "created_at"
	SortFieldProductUpdatedAt SortFieldProduct = "updated_at"
	SortFieldProductName      SortFieldProduct = "name"
)

type SearchProductInput struct {
	TenantID string                                                `json:"tenant_id" validate:"required,notblank"`
	Request  search.Request[SearchProductFilter, SortFieldProduct] `json:"request"   validate:"required"`
}

type GetProductInput struct {
	TenantID  string `json:"tenant_id"  validate:"required,notblank"`
	ProductID string `json:"product_id" validate:"required,notblank"`
}

type DeleteProductInput struct {
	TenantID  string `json:"tenant_id"  validate:"required,notblank"`
	ProductID string `json:"product_id" validate:"required,notblank"`
}

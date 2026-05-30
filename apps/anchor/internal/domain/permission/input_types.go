package permission

import "github.com/nanostack-dev/nanostack-framework/pkg/search"

// CreateProductPermissionInput represents input for creating a new permission.
type CreateProductPermissionInput struct {
	ProductID   string  `json:"product_id"  validate:"required,notblank"`
	Name        string  `json:"name"        validate:"required,permission_name,max=100"`
	Description *string `json:"description" validate:"omitempty,max=255"`
}

// UpdateProductPermissionInput represents input for updating a permission
// Note: Name is immutable, only description can be updated.
type UpdateProductPermissionInput struct {
	ProductID   string  `json:"product_id"  validate:"required,notblank"`
	Name        string  `json:"name"        validate:"required,permission_name,notblank"`
	Description *string `json:"description" validate:"omitempty,max=255"`
}

// DeleteProductPermissionInput represents input for deleting a permission.
type DeleteProductPermissionInput struct {
	ProductID string `json:"product_id" validate:"required,notblank"`
	Name      string `json:"name"       validate:"required,permission_name"`
}

// FindProductPermissionInput represents input for finding a permission.
type FindProductPermissionInput struct {
	ProductID string `json:"product_id" validate:"required,notblank"`
	Name      string `json:"name"       validate:"required,permission_name"`
}

// SearchProductPermissionFilter represents search filter criteria.
type SearchProductPermissionFilter struct {
	Names []string `json:"names,omitempty" validate:"omitempty,dive,permission_name"`
}

// SortFieldProductPermission defines sortable fields.
type SortFieldProductPermission string

const (
	SortFieldProductPermissionCreatedAt SortFieldProductPermission = "created_at"
	SortFieldProductPermissionUpdatedAt SortFieldProductPermission = "updated_at"
	SortFieldProductPermissionName      SortFieldProductPermission = "name"
)

// SearchProductPermissionInput represents search input.
type SearchProductPermissionInput struct {
	ProductID string                                                                    `json:"product_id" validate:"required,notblank"`
	Request   search.Request[SearchProductPermissionFilter, SortFieldProductPermission] `json:"request"    validate:"required"`
}

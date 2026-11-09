package permission

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

// SearchProductPermissionNames is the shape the permission-name check of a
// search filter validates.
type SearchProductPermissionNames struct {
	Names []string `json:"names,omitempty" validate:"omitempty,dive,permission_name"`
}

// SortFieldProductPermission is the sort_by type of POST /permissions/search.
// The generated search owns the values; this type gives the OpenAPI schema its Go type.
type SortFieldProductPermission string

package resourcepermission

import "github.com/nanostack-dev/shared/toolkit/search"

type CreateProductResourcePermissionInput struct {
	ProductID     string  `validate:"required,notblank"`
	Name          string  `validate:"required,notblank,min=2,max=200"`
	Description   *string `validate:"omitempty,max=500"`
	ScopeModifier *string `validate:"omitempty,max=100"`
}

type GetProductResourcePermissionInput struct {
	ProductID      string `validate:"required,notblank"`
	PermissionName string `validate:"required,notblank"`
}

type UpdateProductResourcePermissionInput struct {
	ProductID   string  `validate:"required,notblank"`
	Name        string  `validate:"required,notblank,max=200"`
	Description *string `validate:"omitempty,max=500"`
}

type DeleteProductResourcePermissionInput struct {
	ProductID string `validate:"required,notblank"`
	Name      string `validate:"required,notblank"`
}

type SearchProductResourcePermissionFilter struct {
	Names          []string `validate:"omitempty,dive,notblank"`
	ScopeModifiers []string `validate:"omitempty,dive,notblank"`
}

type SortFieldProductResourcePermission string

const (
	SortFieldProductResourcePermissionCreatedAt SortFieldProductResourcePermission = "created_at"
	SortFieldProductResourcePermissionUpdatedAt SortFieldProductResourcePermission = "updated_at"
	SortFieldProductResourcePermissionName      SortFieldProductResourcePermission = "name"
)

type SearchProductResourcePermissionInput struct {
	ProductID string                                                                                    `validate:"required,notblank"`
	Request   search.Request[SearchProductResourcePermissionFilter, SortFieldProductResourcePermission] `validate:"required"`
}

type GetProductRoleResourcePermissionsInput struct {
	ProductID     string `json:"product_id"      validate:"required,notblank"`
	ProductRoleID string `json:"product_role_id" validate:"required,notblank"`
}

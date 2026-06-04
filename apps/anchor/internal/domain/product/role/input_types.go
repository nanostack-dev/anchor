package role

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

type CreateProductRoleInput struct {
	ProductID   string `validate:"required,notblank"`
	Name        string `validate:"required,notblank,max=100"`
	Description string `validate:"max=500"`
	Permissions []string
}

type GetProductRoleInput struct {
	ProductID string `validate:"required,notblank"`
	ID        string `validate:"required,notblank"`
}

type UpdateProductRoleInput struct {
	ProductID   string  `validate:"required,notblank"`
	ID          string  `validate:"required,notblank"`
	Name        *string `validate:"required,notblank,max=100"`
	Description *string `validate:"omitempty,notblank,max=500"`
	Permissions []ProductRolePermission
}

type DeleteProductRoleInput struct {
	ProductID string `validate:"required,notblank"`
	ID        string `validate:"required,notblank"`
}

type SortFieldProductRole string

const (
	SortFieldProductRoleCreatedAt SortFieldProductRole = "created_at"
	SortFieldProductRoleUpdatedAt SortFieldProductRole = "updated_at"
	SortFieldProductRoleName      SortFieldProductRole = "name"
)

type SearchProductRolesInput struct {
	ProductID string `validate:"required,notblank"`
	Request   search.Request[SearchProductRoleFilter, SortFieldProductRole]
}

type SearchProductRoleFilter struct {
	ProductRoleIDs []string `validate:"omitempty,dive"`
	Names          []string `validate:"omitempty,dive,min=1"`
}

type AssignPermissionToProductRoleInput struct {
	ProductID      string `validate:"required,notblank"`
	ProductRoleID  string `validate:"required,notblank"`
	PermissionName string `validate:"required,notblank,max=100"`
}

type UnassignPermissionFromProductRoleInput struct {
	ProductID      string `validate:"required,notblank"`
	ProductRoleID  string `validate:"required,notblank"`
	PermissionName string `validate:"required,notblank,max=100"`
}

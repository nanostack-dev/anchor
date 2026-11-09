package resourcepermission

type CreateProductResourcePermissionInput struct {
	ProductID     string  `validate:"required,notblank"`
	Name          string  `validate:"required,notblank,min=2,max=200"`
	Description   *string `validate:"omitempty,max=500"`
	ScopeModifier *string `validate:"omitempty,max=100"`
	ResourceType  string  `validate:"max=49"`
}

type GetProductResourcePermissionInput struct {
	ProductID      string `validate:"required,notblank"`
	PermissionName string `validate:"required,notblank"`
}

type UpdateProductResourcePermissionInput struct {
	ProductID    string  `validate:"required,notblank"`
	Name         string  `validate:"required,notblank,max=200"`
	Description  *string `validate:"omitempty,max=500"`
	ResourceType *string `validate:"omitempty,max=49"`
}

type DeleteProductResourcePermissionInput struct {
	ProductID string `validate:"required,notblank"`
	Name      string `validate:"required,notblank"`
}

// SortFieldProductResourcePermission is the sort_by type of POST
// /resource-permissions/search. The generated search owns the values; this type
// gives the OpenAPI schema its Go type.
type SortFieldProductResourcePermission string

type GetProductRoleResourcePermissionsInput struct {
	ProductID     string `json:"product_id"      validate:"required,notblank"`
	ProductRoleID string `json:"product_role_id" validate:"required,notblank"`
}

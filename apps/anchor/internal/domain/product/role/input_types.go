package role

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/resourcetype"

	"anchor/internal/searchx"
)

type CreateProductRoleInput struct {
	ProductID    string `validate:"required,notblank"`
	Name         string `validate:"required,notblank,max=100"`
	Description  string `validate:"max=500"`
	ResourceType string `validate:"max=49"`
	Permissions  []string
}

type GetProductRoleInput struct {
	ProductID string `validate:"required,notblank"`
	ID        string `validate:"required,notblank"`
}

type UpdateProductRoleInput struct {
	ProductID    string  `validate:"required,notblank"`
	ID           string  `validate:"required,notblank"`
	Name         *string `validate:"required,notblank,max=100"`
	Description  *string `validate:"omitempty,notblank,max=500"`
	ResourceType *string `validate:"omitempty,max=49"`
	Permissions  []ProductRolePermission
}

type DeleteProductRoleInput struct {
	ProductID string `validate:"required,notblank"`
	ID        string `validate:"required,notblank"`
}

type SortFieldProductRole string

const (
	SortFieldProductRoleID        SortFieldProductRole = "id"
	SortFieldProductRoleCreatedAt SortFieldProductRole = "created_at"
	SortFieldProductRoleUpdatedAt SortFieldProductRole = "updated_at"
	SortFieldProductRoleName      SortFieldProductRole = "name"
)

type SearchProductRolesInput struct {
	ProductID string `validate:"required,notblank"`
	Request   search.Request[SearchProductRoleFilter, SortFieldProductRole]
}

// Validate checks the input and the sort the client named.
func (in SearchProductRolesInput) Validate() error {
	return searchx.ValidateRequest(in, in.Request,
		SortFieldProductRoleID,
		SortFieldProductRoleCreatedAt,
		SortFieldProductRoleUpdatedAt,
		SortFieldProductRoleName)
}

type SearchProductRoleFilter struct {
	ProductRoleIDs []string `validate:"omitempty,dive"`
	Names          []string `validate:"omitempty,dive,min=1"`
	Permissions    []string `validate:"omitempty,dive,notblank,max=200"`
	ResourceTypes  []string `validate:"omitempty,max=50,dive,notblank"`
	// Types narrows an Organization's role search to managed or custom roles.
	Types []Type `validate:"omitempty,max=2"`
	// Scopes is what the service resolves ResourceTypes to.
	Scopes resourcetype.ScopeFilter
}

// OrganizationRoleRef names a role through an Organization's role routes.
type OrganizationRoleRef struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	ID             string `validate:"required,notblank"`
}

type CreateOrganizationRoleInput struct {
	OrganizationID string `validate:"required,notblank"`
	CreateProductRoleInput
}

type UpdateOrganizationRoleInput struct {
	OrganizationID string `validate:"required,notblank"`
	UpdateProductRoleInput
}

type SearchOrganizationRolesInput struct {
	OrganizationID string `validate:"required,notblank"`
	SearchProductRolesInput
}

// Validate checks the input and the sort the client named.
func (in SearchOrganizationRolesInput) Validate() error {
	if err := searchx.ValidateRequest(in, in.Request,
		SortFieldProductRoleID,
		SortFieldProductRoleCreatedAt,
		SortFieldProductRoleUpdatedAt,
		SortFieldProductRoleName); err != nil {
		return err
	}
	if in.Request.Filter == nil {
		return nil
	}
	types := make([]string, 0, len(in.Request.Filter.Types))
	for _, t := range in.Request.Filter.Types {
		types = append(types, string(t))
	}
	return searchx.ValidateOneOf("filter.types", types, []string{string(TypeManaged), string(TypeCustom)})
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

// WithScopes resolves the filter's resource types against the product's tree.
func (f SearchProductRoleFilter) WithScopes(tree resourcetype.Tree) SearchProductRoleFilter {
	f.Scopes = tree.ResolveScopes(f.ResourceTypes)
	return f
}

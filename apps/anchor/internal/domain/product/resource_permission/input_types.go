package resourcepermission

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/resourcetype"

	"anchor/internal/searchx"
)

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

type SearchProductResourcePermissionFilter struct {
	Names          []string `validate:"omitempty,dive,notblank"`
	ScopeModifiers []string `validate:"omitempty,dive,notblank"`
	ResourceTypes  []string `validate:"omitempty,max=50,dive,notblank"`
	// Scopes is what the service resolves ResourceTypes to.
	Scopes resourcetype.ScopeFilter
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

// Validate checks the input and the sort the client named.
func (in SearchProductResourcePermissionInput) Validate() error {
	return searchx.ValidateRequest(in, in.Request,
		SortFieldProductResourcePermissionCreatedAt,
		SortFieldProductResourcePermissionUpdatedAt,
		SortFieldProductResourcePermissionName)
}

// WithScopes resolves the filter's resource types against the product's tree.
func (f SearchProductResourcePermissionFilter) WithScopes(
	tree resourcetype.Tree,
) SearchProductResourcePermissionFilter {
	f.Scopes = tree.ResolveScopes(f.ResourceTypes)
	return f
}

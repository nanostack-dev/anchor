//revive:disable-next-line:var-naming
package user

import (
	"github.com/nanostack-dev/nanostack-framework/db/search"

	"anchor/internal/searchx"
)

type CreateProductUserInput struct {
	ProductID  string            `validate:"required,notblank"`
	Email      string            `validate:"required,email"`
	Name       *string           `validate:"omitempty,notblank"`
	Status     ProductUserStatus `validate:"required,oneof=active invited suspended"`
	ExternalID *string           `validate:"omitempty,notblank,max=255,printascii"`
}

type UpdateProductUserInput struct {
	ProductID     string  `validate:"required,notblank"`
	ProductUserID string  `validate:"required,notblank"`
	Email         *string `validate:"omitempty,email"`
	Name          *string `validate:"omitempty,notblank"`
	Status        *ProductUserStatus
}

type FindProductUserInput struct {
	ProductID     string `validate:"required,notblank"`
	ProductUserID string `validate:"required,notblank"`
}

type FindProductUserByExternalIDInput struct {
	ProductID  string `validate:"required,notblank"`
	ExternalID string `validate:"required,notblank"`
}

type DeleteProductUserInput struct {
	ProductID     string `validate:"required,notblank"`
	ProductUserID string `validate:"required,notblank"`
}

// SortFieldProductUser is the sort_by type of POST /product-users/search. The
// generated search owns the values; this type gives the OpenAPI schema its Go type.
type SortFieldProductUser string

// ListUserOrganizationsInput is the input for listing organizations a user belongs to.
type ListUserOrganizationsInput struct {
	ProductID          string `validate:"required,notblank"`
	ProductUserID      string `validate:"required,notblank"`
	IncludePermissions bool
}

// SortFieldUserOrganization names a column a user-organization search may order by.
type SortFieldUserOrganization string

const (
	SortFieldUserOrganizationJoinedAt SortFieldUserOrganization = "joined_at"
	SortFieldUserOrganizationName     SortFieldUserOrganization = "name"
)

// SearchUserOrganizationsFilter narrows the organizations a user belongs to.
type SearchUserOrganizationsFilter struct {
	OrganizationIDs []string
	RoleIDs         []string
}

// SearchUserOrganizationsInput is the input for searching the organizations a
// product user belongs to.
type SearchUserOrganizationsInput struct {
	ProductID          string `validate:"required,notblank"`
	ProductUserID      string `validate:"required,notblank"`
	IncludePermissions bool
	Request            search.Request[SearchUserOrganizationsFilter, SortFieldUserOrganization] `validate:"required"`
}

// Validate checks the input and the sort the client named.
func (in SearchUserOrganizationsInput) Validate() error {
	return searchx.ValidateRequest(in, in.Request,
		SortFieldUserOrganizationJoinedAt,
		SortFieldUserOrganizationName)
}

// GetUserOrganizationInput is the input for getting a specific organization a user belongs to.
type GetUserOrganizationInput struct {
	ProductID          string `validate:"required,notblank"`
	ProductUserID      string `validate:"required,notblank"`
	OrganizationID     string `validate:"required,notblank"`
	IncludePermissions bool
}

// CreateOrganizationMembershipInput is the input for creating a user organization membership.
type CreateOrganizationMembershipInput struct {
	ProductID      string `validate:"required,notblank"`
	ProductUserID  string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	RoleID         string `validate:"required,notblank"`
}

// UpdateOrganizationMembershipInput is the input for setting a user organization membership role.
type UpdateOrganizationMembershipInput struct {
	ProductID      string `validate:"required,notblank"`
	ProductUserID  string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	RoleID         string `validate:"required,notblank"`
}

// DeleteOrganizationMembershipInput is the input for deleting a user organization membership.
type DeleteOrganizationMembershipInput struct {
	ProductID      string `validate:"required,notblank"`
	ProductUserID  string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
}

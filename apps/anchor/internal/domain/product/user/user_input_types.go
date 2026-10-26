//revive:disable-next-line:var-naming
package user

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/searchx"
)

type CreateProductUserInput struct {
	ProductID  string            `validate:"required,notblank"`
	Email      string            `validate:"required,email"`
	Name       *string           `validate:"omitempty,notblank,max=100"`
	Status     ProductUserStatus `validate:"required"`
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

type SearchProductUserFilter struct {
	IDs         []string            `validate:"omitempty,dive,notblank"`
	Emails      []string            `validate:"omitempty,dive,email"`
	Names       []string            `validate:"omitempty,dive,notblank"`
	Statuses    []ProductUserStatus `validate:"omitempty,dive,oneof=active invited suspended ACTIVE INACTIVE"`
	ExternalIDs []string            `validate:"omitempty,dive,notblank"`
}

type SortFieldProductUser string

const (
	SortFieldProductUserCreatedAt SortFieldProductUser = "created_at"
	SortFieldProductUserUpdatedAt SortFieldProductUser = "updated_at"
	SortFieldProductUserEmail     SortFieldProductUser = "email"
	SortFieldProductUserName      SortFieldProductUser = "name"
	SortFieldProductUserStatus    SortFieldProductUser = "status"
)

type SearchProductUserInput struct {
	ProductID string                                                        `validate:"required,notblank"`
	Request   search.Request[SearchProductUserFilter, SortFieldProductUser] `validate:"required"`
}

// Validate checks the input and the sort the client named.
func (in SearchProductUserInput) Validate() error {
	return searchx.ValidateRequest(in, in.Request,
		SortFieldProductUserCreatedAt,
		SortFieldProductUserUpdatedAt,
		SortFieldProductUserEmail,
		SortFieldProductUserName,
		SortFieldProductUserStatus)
}

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

//revive:disable-next-line:var-naming
package user

import "github.com/nanostack-dev/nanostack-framework/pkg/search"

type CreateProductUserInput struct {
	ProductID string            `validate:"required,notblank"`
	Email     string            `validate:"required,email"`
	Name      string            `validate:"required,notblank"`
	Status    ProductUserStatus `validate:"required"`
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
	IDs         []string `validate:"omitempty,dive,notblank"`
	Emails      []string `validate:"omitempty,dive,email"`
	Names       []string `validate:"omitempty,dive,notblank"`
	Statuses    []ProductUserStatus
	ExternalIDs []string `validate:"omitempty,dive,notblank"`
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

// ListUserOrganizationsInput is the input for listing organizations a user belongs to.
type ListUserOrganizationsInput struct {
	ProductID          string `validate:"required,notblank"`
	ProductUserID      string `validate:"required,notblank"`
	IncludePermissions bool
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

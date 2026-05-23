package organization

import "github.com/nanostack-dev/nanostack-framework/pkg/search"

// AddMemberInput is the input for adding a product user to an organization with a role.
type AddMemberInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	ProductUserID  string `validate:"required,notblank"`
	RoleID         string `validate:"required,notblank"`
}

// UpdateMemberRoleInput is the input for updating a member's role within an organization.
type UpdateMemberRoleInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	ProductUserID  string `validate:"required,notblank"`
	RoleID         string `validate:"required,notblank"`
}

// RemoveMemberInput is the input for removing a product user from an organization.
type RemoveMemberInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	ProductUserID  string `validate:"required,notblank"`
}

// GetMemberInput is the input for retrieving a specific member of an organization.
type GetMemberInput struct {
	ProductID          string `validate:"required,notblank"`
	OrganizationID     string `validate:"required,notblank"`
	ProductUserID      string `validate:"required,notblank"`
	IncludePermissions bool
}

// ListMembersInput is the input for listing all members of an organization.
type ListMembersInput struct {
	ProductID          string `validate:"required,notblank"`
	OrganizationID     string `validate:"required,notblank"`
	IncludePermissions bool
}

// SearchMembersFilter defines filterable fields when searching organization members.
type SearchMembersFilter struct {
	ProductUserIDs []string `validate:"omitempty,dive,notblank"`
	ExternalIDs    []string `validate:"omitempty,dive,notblank"` // Clerk user IDs
	Emails         []string `validate:"omitempty,dive,email"`
	RoleIDs        []string `validate:"omitempty,dive,notblank"`
}

// SortFieldMember defines valid sort fields for member search results.
type SortFieldMember string

const (
	SortFieldMemberJoinedAt SortFieldMember = "joined_at"
	SortFieldMemberEmail    SortFieldMember = "email"
)

// SearchMembersInput is the input for searching members within an organization.
type SearchMembersInput struct {
	ProductID      string                                               `validate:"required,notblank"`
	OrganizationID string                                               `validate:"required,notblank"`
	Request        search.Request[SearchMembersFilter, SortFieldMember] `validate:"required"`
}

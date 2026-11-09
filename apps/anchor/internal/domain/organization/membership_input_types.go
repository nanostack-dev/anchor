package organization

// AddMemberInput is the input for adding a product user to an organization.
// A nil RoleID adds a member who holds no role.
type AddMemberInput struct {
	ProductID      string  `validate:"required,notblank"`
	OrganizationID string  `validate:"required,notblank"`
	ProductUserID  string  `validate:"required,notblank"`
	RoleID         *string `validate:"omitempty,notblank"`
}

// UpdateMemberRoleInput is the input for updating a member's role within an
// organization. A nil RoleID removes the member's role.
type UpdateMemberRoleInput struct {
	ProductID      string  `validate:"required,notblank"`
	OrganizationID string  `validate:"required,notblank"`
	ProductUserID  string  `validate:"required,notblank"`
	RoleID         *string `validate:"omitempty,notblank"`
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

// SortFieldMember is the sort_by type of POST /organizations/{id}/members/search.
// The generated search owns the values; this type gives the OpenAPI schema its Go type.
type SortFieldMember string

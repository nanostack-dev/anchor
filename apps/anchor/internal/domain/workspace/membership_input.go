package workspace

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/searchx"
)

// AssignMemberRoleInput is the input for giving an organization member a role in
// a workspace, or replacing the role they hold there.
type AssignMemberRoleInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	WorkspaceID    string `validate:"required,notblank"`
	ProductUserID  string `validate:"required,notblank"`
	RoleID         string `validate:"required,notblank"`
}

// GetMemberInput is the input for reading the role a member holds in a workspace.
type GetMemberInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	WorkspaceID    string `validate:"required,notblank"`
	ProductUserID  string `validate:"required,notblank"`
}

// RemoveMemberInput is the input for removing the role a member holds in a workspace.
type RemoveMemberInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	WorkspaceID    string `validate:"required,notblank"`
	ProductUserID  string `validate:"required,notblank"`
}

// SearchMembersFilter defines filterable fields when searching workspace members.
type SearchMembersFilter struct {
	ProductUserIDs []string `validate:"omitempty,dive,notblank"`
	RoleIDs        []string `validate:"omitempty,dive,notblank"`
}

// SearchMembersInput is the input for searching the members of one workspace.
type SearchMembersInput struct {
	ProductID      string                                               `validate:"required,notblank"`
	OrganizationID string                                               `validate:"required,notblank"`
	WorkspaceID    string                                               `validate:"required,notblank"`
	Request        search.Request[SearchMembersFilter, SortFieldMember] `validate:"required"`
}

// Validate checks the input and the sort the client named.
func (in SearchMembersInput) Validate() error {
	return searchx.ValidateRequest(in, in.Request,
		SortFieldMemberAssignedAt,
		SortFieldMemberEmail)
}

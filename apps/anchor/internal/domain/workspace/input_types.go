package workspace

import "github.com/nanostack-dev/nanostack-framework/pkg/search"

type CreateWorkspaceInput struct {
	ProductID      string  `validate:"required,notblank"`
	OrganizationID string  `validate:"required,notblank"`
	Name           string  `validate:"required,notblank,min=2,max=100"`
	Description    *string `validate:"omitempty,max=500"`
}

type FindWorkspaceInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	WorkspaceID    string `validate:"required,notblank"`
}

type UpdateWorkspaceInput struct {
	ProductID      string  `validate:"required,notblank"`
	OrganizationID string  `validate:"required,notblank"`
	WorkspaceID    string  `validate:"required,notblank"`
	Name           *string `validate:"omitempty,notblank,min=2,max=100"`
	Description    *string `validate:"omitempty,max=500"`
}

type DeleteWorkspaceInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	WorkspaceID    string `validate:"required,notblank"`
}

type SearchWorkspaceFilter struct {
	IDs   []string `validate:"omitempty,dive,notblank"`
	Names []string `validate:"omitempty,dive,notblank"`
}

type SortFieldProductWorkspace string

const (
	SortFieldProductWorkspaceCreatedAt SortFieldProductWorkspace = "created_at"
	SortFieldProductWorkspaceUpdatedAt SortFieldProductWorkspace = "updated_at"
	SortFieldProductWorkspaceName      SortFieldProductWorkspace = "name"
)

type SearchWorkspacesInput struct {
	ProductID      string                                                           `validate:"required,notblank"`
	OrganizationID string                                                           `validate:"required,notblank"`
	Request        search.Request[SearchWorkspaceFilter, SortFieldProductWorkspace] `validate:"required"`
}

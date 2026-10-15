package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/workspace"
)

type WorkspaceMapper struct{}

func NewWorkspaceMapper() *WorkspaceMapper {
	return &WorkspaceMapper{}
}

func (m *WorkspaceMapper) ToDomain(entity model.Workspaces) workspace.Workspace {
	return workspace.Workspace{
		ID:             entity.ID,
		OrganizationID: entity.OrganizationID,
		Name:           entity.Name,
		Description:    entity.Description,
		CreatedAt:      entity.CreatedAt,
		UpdatedAt:      entity.UpdatedAt,
	}
}

func (m *WorkspaceMapper) ToEntity(domain workspace.Workspace) model.Workspaces {
	return model.Workspaces{
		ID:             domain.ID,
		OrganizationID: domain.OrganizationID,
		Name:           domain.Name,
		Description:    domain.Description,
		CreatedAt:      domain.CreatedAt,
		UpdatedAt:      domain.UpdatedAt,
	}
}

// Package mapper contains mapping functions between domain and entity types
package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/invitation"
)

type InvitationMapper struct{}

func NewInvitationMapper() *InvitationMapper {
	return &InvitationMapper{}
}

func (m *InvitationMapper) ToDomain(entity model.PlatformInvitations) invitation.PlatformInvitation {
	return invitation.PlatformInvitation{
		ID:               entity.ID,
		Code:             entity.Code,
		Email:            entity.Email,
		PlatformTenantID: entity.PlatformTenantID,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func (m *InvitationMapper) ToDomainList(entities []model.PlatformInvitations) []invitation.PlatformInvitation {
	domains := make([]invitation.PlatformInvitation, len(entities))
	for i, entity := range entities {
		domains[i] = m.ToDomain(entity)
	}
	return domains
}

func (m *InvitationMapper) ToEntity(domain invitation.PlatformInvitation) model.PlatformInvitations {
	return model.PlatformInvitations{
		ID:               domain.ID,
		Code:             domain.Code,
		Email:            domain.Email,
		PlatformTenantID: domain.PlatformTenantID,
		CreatedAt:        domain.CreatedAt,
		UpdatedAt:        domain.UpdatedAt,
	}
}

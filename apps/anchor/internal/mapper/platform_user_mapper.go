package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/platform"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

type PlatformUserMapper struct{}

func NewPlatformUserMapper() *PlatformUserMapper {
	return &PlatformUserMapper{}
}

func (m *PlatformUserMapper) ToDomain(entity model.PlatformUsers) platform.User {
	name := functional.FromPtr(entity.Name).OrElse("")

	return platform.User{
		ID:               entity.ID,
		ExternalID:       entity.ExternalID,
		UserID:           entity.UserID,
		HashedPassword:   entity.HashedPassword,
		Name:             name,
		Email:            entity.Email,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
		PlatformTenantID: entity.PlatformTenantID,
		Role:             platform.TenantRole(entity.Role),
	}
}

func (m *PlatformUserMapper) ToEntity(domain platform.User) model.PlatformUsers {
	name := functional.OptionOf(domain.Name, domain.Name != "").ToPtr()

	return model.PlatformUsers{
		ID:               domain.ID,
		UserID:           domain.UserID,
		ExternalID:       domain.ExternalID,
		Email:            domain.Email,
		Name:             name,
		HashedPassword:   domain.HashedPassword,
		PlatformTenantID: domain.PlatformTenantID,
		Role:             domain.Role.ToString(),
		CreatedAt:        domain.CreatedAt,
		UpdatedAt:        domain.UpdatedAt,
	}
}

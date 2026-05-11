package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/auth"
)

type UserMapper struct{}

func NewUserMapper() *UserMapper {
	return &UserMapper{}
}
func (m *UserMapper) ToDomain(
	entity model.Users,
) auth.User {
	return auth.User{
		ID:             entity.ID,
		Email:          entity.Email,
		HashedPassword: entity.HashedPassword,
		ExternalID:     entity.ExternalID,
		CreatedAt:      entity.CreatedAt,
		UpdatedAt:      entity.UpdatedAt,
	}
}

func (m *UserMapper) ToEntity(domain auth.User) model.Users {
	return model.Users{
		ID:             domain.ID,
		Email:          domain.Email,
		ExternalID:     domain.ExternalID,
		HashedPassword: domain.HashedPassword,
		CreatedAt:      domain.CreatedAt,
		UpdatedAt:      domain.UpdatedAt,
	}
}

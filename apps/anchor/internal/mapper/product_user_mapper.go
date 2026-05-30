package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/product/user"
)

type ProductUserMapper struct{}

// Removed static interface check

func NewProductUserMapper() *ProductUserMapper { // Return concrete type
	return &ProductUserMapper{}
}

func (m *ProductUserMapper) ToDomain(entity model.ProductUsers) user.ProductUser {
	var name string
	if entity.Name != nil {
		name = *entity.Name
	}

	return user.ProductUser{
		ID:         entity.ID,
		ProductID:  entity.ProductID,
		Email:      entity.Email,
		Name:       name,
		ExternalID: entity.ExternalID,
		Status:     user.ProductUserStatus(entity.Status),
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
	}
}

func (m *ProductUserMapper) ToEntity(domain user.ProductUser) model.ProductUsers {
	var name *string
	if domain.Name != "" {
		n := domain.Name
		name = &n
	}

	return model.ProductUsers{
		ID:         domain.ID,
		ProductID:  domain.ProductID,
		Email:      domain.Email,
		Name:       name,
		ExternalID: domain.ExternalID,
		Status:     string(domain.Status),
		CreatedAt:  domain.CreatedAt,
		UpdatedAt:  domain.UpdatedAt,
	}
}

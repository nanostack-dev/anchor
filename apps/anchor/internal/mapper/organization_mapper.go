package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/organization"
)

type OrganizationMapper struct{}

func NewOrganizationMapper() *OrganizationMapper {
	return &OrganizationMapper{}
}

func (m *OrganizationMapper) ToDomain(entity model.Organizations) organization.Organization {
	return organization.Organization{
		ID:          entity.ID,
		ProductID:   entity.ProductID,
		Name:        entity.Name,
		Description: entity.Description,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}

func (m *OrganizationMapper) ToEntity(domain organization.Organization) model.Organizations {
	return model.Organizations{
		ID:          domain.ID,
		ProductID:   domain.ProductID,
		Name:        domain.Name,
		Description: domain.Description,
		CreatedAt:   domain.CreatedAt,
		UpdatedAt:   domain.UpdatedAt,
	}
}

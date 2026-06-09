package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/product"
)

type ProductMapper struct{}

func NewProductMapper() *ProductMapper {
	return &ProductMapper{}
}

func (m *ProductMapper) ToDomain(
	entity model.Products,
	organizationAPIKeyConfig model.ProductOrganizationAPIKeyConfigs,
) product.Product {
	var description string
	if entity.Description != nil {
		description = *entity.Description
	}

	config := product.Config{
		OrganizationAPIKeys: product.OrganizationAPIKeysConfig{Prefix: organizationAPIKeyConfig.Prefix},
	}.WithDefaults()

	return product.Product{
		ID:               entity.ID,
		PlatformTenantID: entity.PlatformTenantID,
		Name:             entity.Name,
		Description:      description,
		Config:           config,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func (m *ProductMapper) ToEntity(domain product.Product) model.Products {
	var description *string
	if domain.Description != "" {
		desc := domain.Description
		description = &desc
	}

	return model.Products{
		ID:               domain.ID,
		PlatformTenantID: domain.PlatformTenantID,
		Name:             domain.Name,
		Description:      description,
		CreatedAt:        domain.CreatedAt,
		UpdatedAt:        domain.UpdatedAt,
	}
}

func (m *ProductMapper) OrganizationAPIKeyConfigToEntity(
	productID string,
	config product.OrganizationAPIKeysConfig,
) model.ProductOrganizationAPIKeyConfigs {
	return model.ProductOrganizationAPIKeyConfigs{
		ProductID: productID,
		Prefix:    config.Prefix,
	}
}

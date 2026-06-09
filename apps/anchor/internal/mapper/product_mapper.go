package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/product"
)

type ProductMapper struct{}

func NewProductMapper() *ProductMapper {
	return &ProductMapper{}
}

func (m *ProductMapper) ToDomain(entity model.Products, apiKeyConfig model.ProductAPIKeyConfigs) product.Product {
	var description string
	if entity.Description != nil {
		description = *entity.Description
	}

	config := product.Config{
		APIKeys: product.APIKeysConfig{Prefix: apiKeyConfig.Prefix},
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

func (m *ProductMapper) APIKeyConfigToEntity(
	productID string,
	config product.APIKeysConfig,
) model.ProductAPIKeyConfigs {
	return model.ProductAPIKeyConfigs{
		ProductID: productID,
		Prefix:    config.Prefix,
	}
}

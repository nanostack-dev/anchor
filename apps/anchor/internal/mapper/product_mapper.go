package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/product"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

type ProductMapper struct{}

func NewProductMapper() *ProductMapper {
	return &ProductMapper{}
}

func (m *ProductMapper) ToDomain(
	entity model.Products,
	organizationAPIKeyConfig model.ProductOrganizationAPIKeyConfigs,
) product.Product {
	description := functional.FromPtr(entity.Description).OrElse("")

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
	description := functional.OptionOf(domain.Description, domain.Description != "").ToPtr()

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

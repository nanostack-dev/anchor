package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	resourcepermission "anchor/internal/domain/product/resource_permission"
)

type ProductResourcePermissionMapper struct{}

func NewProductResourcePermissionMapper() *ProductResourcePermissionMapper {
	return &ProductResourcePermissionMapper{}
}

func (m *ProductResourcePermissionMapper) ToDomain(
	entity model.ProductResourcePermissions,
) resourcepermission.ProductResourcePermission {
	return resourcepermission.ProductResourcePermission{
		ProductID:     entity.ProductID,
		Name:          entity.Name,
		Description:   entity.Description,
		ScopeModifier: entity.ScopeModifier,
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
	}
}

func (m *ProductResourcePermissionMapper) ToEntity(
	domain resourcepermission.ProductResourcePermission,
) model.ProductResourcePermissions {
	return model.ProductResourcePermissions{
		ProductID:     domain.ProductID,
		Name:          domain.Name,
		Description:   domain.Description,
		ScopeModifier: domain.ScopeModifier,
		CreatedAt:     domain.CreatedAt,
		UpdatedAt:     domain.UpdatedAt,
	}
}

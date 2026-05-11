package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/permission"
)

type ProductPermissionMapper struct{}

func NewProductPermissionMapper() *ProductPermissionMapper {
	return &ProductPermissionMapper{}
}

func (m *ProductPermissionMapper) ToDomain(entity model.ProductPermissions) permission.ProductPermission {
	var description *string
	if entity.Description != nil {
		description = entity.Description
	}

	return permission.ProductPermission{
		ProductID:   entity.ProductID,
		Name:        entity.Name,
		Description: description,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}

func (m *ProductPermissionMapper) ToEntity(domain permission.ProductPermission) model.ProductPermissions {
	var description *string
	if domain.Description != nil {
		description = domain.Description
	}

	return model.ProductPermissions{
		ProductID:   domain.ProductID,
		Name:        domain.Name,
		Description: description,
		CreatedAt:   domain.CreatedAt,
		UpdatedAt:   domain.UpdatedAt,
	}
}

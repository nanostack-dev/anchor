package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/product/role"
)

type ProductRoleMapper struct{}

func NewProductRoleMapper() *ProductRoleMapper {
	return &ProductRoleMapper{}
}

func (m *ProductRoleMapper) ToDomain(
	entity model.ProductRoles, permissions []model.ProductRoleResourcePermissions,
) role.ProductRole {
	var description string
	if entity.Description != nil {
		description = *entity.Description
	}

	return role.ProductRole{
		ID:          entity.ID,
		ProductID:   entity.ProductID,
		Name:        entity.Name,
		Description: description,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
		Permissions: m.permissionsToDomain(permissions),
	}
}

func (m *ProductRoleMapper) ToEntity(domain role.ProductRole) model.ProductRoles {
	var description *string
	if domain.Description != "" {
		desc := domain.Description
		description = &desc
	}

	return model.ProductRoles{
		ID:          domain.ID,
		ProductID:   domain.ProductID,
		Name:        domain.Name,
		Description: description,
		CreatedAt:   domain.CreatedAt,
		UpdatedAt:   domain.UpdatedAt,
	}
}

func (m *ProductRoleMapper) permissionsToDomain(
	permissions []model.ProductRoleResourcePermissions,
) []role.ProductRolePermission {
	if permissions == nil {
		return nil
	}

	domainPermissions := make([]role.ProductRolePermission, len(permissions))
	for i, perm := range permissions {
		domainPermissions[i] = role.ProductRolePermission{
			ID:             perm.ID,
			ProductRoleID:  perm.ProductRoleID,
			ProductID:      perm.ProductID,
			PermissionName: perm.PermissionName,
		}
	}
	return domainPermissions
}

func (m *ProductRoleMapper) PermissionsToEntities(
	permissions []role.ProductRolePermission,
) []model.ProductRoleResourcePermissions {
	if permissions == nil {
		return nil
	}

	entities := make([]model.ProductRoleResourcePermissions, len(permissions))
	for i, perm := range permissions {
		entities[i] = model.ProductRoleResourcePermissions{
			ID:             perm.ID,
			ProductRoleID:  perm.ProductRoleID,
			ProductID:      perm.ProductID,
			PermissionName: perm.PermissionName,
		}
	}
	return entities
}

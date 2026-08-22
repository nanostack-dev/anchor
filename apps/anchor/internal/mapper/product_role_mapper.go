package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/product/role"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

type ProductRoleMapper struct{}

func NewProductRoleMapper() *ProductRoleMapper {
	return &ProductRoleMapper{}
}

func (m *ProductRoleMapper) ToDomain(
	entity model.ProductRoles, permissions []model.ProductRoleResourcePermissions,
) role.ProductRole {
	description := functional.FromPtr(entity.Description).OrElse("")

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
	description := functional.OptionOf(domain.Description, domain.Description != "").ToPtr()

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
	return functional.Slice(permissions).
		Map(func(perm model.ProductRoleResourcePermissions) role.ProductRolePermission {
			return role.ProductRolePermission{
				ID:             perm.ID,
				ProductRoleID:  perm.ProductRoleID,
				ProductID:      perm.ProductID,
				PermissionName: perm.PermissionName,
			}
		})
}

func (m *ProductRoleMapper) PermissionsToEntities(
	permissions []role.ProductRolePermission,
) []model.ProductRoleResourcePermissions {
	return functional.Slice(permissions).
		Map(func(perm role.ProductRolePermission) model.ProductRoleResourcePermissions {
			return model.ProductRoleResourcePermissions{
				ID:             perm.ID,
				ProductRoleID:  perm.ProductRoleID,
				ProductID:      perm.ProductID,
				PermissionName: perm.PermissionName,
			}
		})
}

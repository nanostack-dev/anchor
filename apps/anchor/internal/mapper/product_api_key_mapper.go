package mapper

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/product/apikey"
)

type ProductAPIKeyMapper struct{}

func NewProductAPIKeyMapper() *ProductAPIKeyMapper {
	return &ProductAPIKeyMapper{}
}

func (m *ProductAPIKeyMapper) ToDomain(entity model.ProductAPIKeys) apikey.ProductAPIKey {
	return apikey.ProductAPIKey{
		ID:              entity.ID,
		ProductID:       entity.ProductID,
		Name:            entity.Name,
		Description:     entity.Description,
		Mutable:         entity.Mutable,
		ObfuscatedValue: entity.ObfuscatedValue,
		HashedValue:     entity.HashedValue,
		LastUsedAt:      entity.LastUsedAt,
		Status:          apikey.Status(entity.Status),
		CreatedAt:       entity.CreatedAt,
		UpdatedAt:       entity.UpdatedAt,
	}
}

func (m *ProductAPIKeyMapper) ToDomainWithPermissions(
	entity model.ProductAPIKeys, permissionEntities []model.ProductAPIKeyPermissions,
) apikey.ProductAPIKey {
	domain := m.ToDomain(entity)
	domain.Permissions = functional.Slice(
		permissionEntities).Map(

		func(perm model.ProductAPIKeyPermissions) apikey.ProductAPIKeyPermission {
			return m.PermissionToDomain(perm)
		})

	return domain
}

func (m *ProductAPIKeyMapper) ToEntity(domain apikey.ProductAPIKey) model.ProductAPIKeys {
	return model.ProductAPIKeys{
		HashedValue:     domain.HashedValue,
		ID:              domain.ID,
		ProductID:       domain.ProductID,
		Name:            domain.Name,
		Description:     domain.Description,
		Mutable:         domain.Mutable,
		ObfuscatedValue: domain.ObfuscatedValue,
		LastUsedAt:      domain.LastUsedAt,
		Status:          string(domain.Status),
		CreatedAt:       domain.CreatedAt,
		UpdatedAt:       domain.UpdatedAt,
	}
}

func (m *ProductAPIKeyMapper) PermissionToEntity(
	permission apikey.ProductAPIKeyPermission,
) model.ProductAPIKeyPermissions {
	return model.ProductAPIKeyPermissions{
		ProductID:      permission.ProductID,
		APIKeyID:       permission.APIKeyID,
		PermissionName: permission.PermissionName,
		CreatedAt:      permission.CreatedAt,
	}
}

func (m *ProductAPIKeyMapper) PermissionToDomain(entity model.ProductAPIKeyPermissions) apikey.ProductAPIKeyPermission {
	return apikey.ProductAPIKeyPermission{
		APIKeyID:       entity.APIKeyID,
		ProductID:      entity.ProductID,
		PermissionName: entity.PermissionName,
		CreatedAt:      entity.CreatedAt,
	}
}

func (m *ProductAPIKeyMapper) PermissionsToDomain(
	entities []model.ProductAPIKeyPermissions,
) []apikey.ProductAPIKeyPermission {
	return functional.Slice(
		entities).Map(

		func(perm model.ProductAPIKeyPermissions) apikey.ProductAPIKeyPermission {
			return m.PermissionToDomain(perm)
		})
}

func (m *ProductAPIKeyMapper) PermissionsToEntity(
	domain []apikey.ProductAPIKeyPermission,
) []model.ProductAPIKeyPermissions {
	return functional.Slice(
		domain).Map(

		func(perm apikey.ProductAPIKeyPermission) model.ProductAPIKeyPermissions {
			return m.PermissionToEntity(perm)
		})
}

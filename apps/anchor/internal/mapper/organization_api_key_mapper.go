package mapper

import (
	"github.com/nanostack-dev/shared/toolkit"

	"anchor/internal/db/gen/anchor/public/model"
	orgapikey "anchor/internal/domain/organization/apikey"
)

type OrganizationAPIKeyMapper struct{}

func NewOrganizationAPIKeyMapper() *OrganizationAPIKeyMapper {
	return &OrganizationAPIKeyMapper{}
}

func (m *OrganizationAPIKeyMapper) ToDomain(entity model.OrganizationAPIKeys) orgapikey.OrganizationAPIKey {
	return orgapikey.OrganizationAPIKey{
		ID:              entity.ID,
		OrganizationID:  entity.OrganizationID,
		Name:            entity.Name,
		Description:     entity.Description,
		ObfuscatedValue: entity.ObfuscatedValue,
		HashedValue:     entity.HashedValue,
		ExpiresAt:       entity.ExpiresAt,
		LastUsedAt:      entity.LastUsedAt,
		Status:          orgapikey.Status(entity.Status),
		CreatedAt:       entity.CreatedAt,
		UpdatedAt:       entity.UpdatedAt,
	}
}

func (m *OrganizationAPIKeyMapper) ToDomainWithPermissions(
	entity model.OrganizationAPIKeys,
	permissionEntities []model.OrganizationAPIKeyPermissions,
) orgapikey.OrganizationAPIKey {
	domain := m.ToDomain(entity)
	domain.Permissions = toolkit.TransformSlice(
		permissionEntities,
		func(perm model.OrganizationAPIKeyPermissions) orgapikey.OrganizationAPIKeyPermission {
			return m.PermissionToDomain(perm)
		},
	)
	return domain
}

func (m *OrganizationAPIKeyMapper) ToEntity(
	domain orgapikey.OrganizationAPIKey,
) model.OrganizationAPIKeys {
	return model.OrganizationAPIKeys{
		ID:              domain.ID,
		OrganizationID:  domain.OrganizationID,
		Name:            domain.Name,
		Description:     domain.Description,
		HashedValue:     domain.HashedValue,
		ObfuscatedValue: domain.ObfuscatedValue,
		Status:          string(domain.Status),
		ExpiresAt:       domain.ExpiresAt,
		LastUsedAt:      domain.LastUsedAt,
		CreatedAt:       domain.CreatedAt,
		UpdatedAt:       domain.UpdatedAt,
	}
}

func (m *OrganizationAPIKeyMapper) PermissionToEntity(
	permission orgapikey.OrganizationAPIKeyPermission,
) model.OrganizationAPIKeyPermissions {
	return model.OrganizationAPIKeyPermissions{
		APIKeyID:       permission.APIKeyID,
		OrganizationID: permission.OrganizationID,
		ProductID:      permission.ProductID,
		PermissionName: permission.PermissionName,
		CreatedAt:      permission.CreatedAt,
	}
}

func (m *OrganizationAPIKeyMapper) PermissionToDomain(
	entity model.OrganizationAPIKeyPermissions,
) orgapikey.OrganizationAPIKeyPermission {
	return orgapikey.OrganizationAPIKeyPermission{
		APIKeyID:       entity.APIKeyID,
		OrganizationID: entity.OrganizationID,
		ProductID:      entity.ProductID,
		PermissionName: entity.PermissionName,
		CreatedAt:      entity.CreatedAt,
	}
}

func (m *OrganizationAPIKeyMapper) PermissionsToEntity(
	domain []orgapikey.OrganizationAPIKeyPermission,
) []model.OrganizationAPIKeyPermissions {
	return toolkit.TransformSlice(
		domain,
		func(perm orgapikey.OrganizationAPIKeyPermission) model.OrganizationAPIKeyPermissions {
			return m.PermissionToEntity(perm)
		},
	)
}

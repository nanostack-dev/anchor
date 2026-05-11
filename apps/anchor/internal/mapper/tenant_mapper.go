package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/tenant"
)

type TenantMapper struct {
}

func NewTenantMapper() *TenantMapper {
	return &TenantMapper{}
}
func (tm *TenantMapper) ToDomain(platformTenant model.PlatformTenants) tenant.PlatformTenant {
	return tenant.PlatformTenant{
		ID:        platformTenant.ID,
		Name:      platformTenant.Name,
		Status:    tenant.Status(platformTenant.Status),
		CreatedAt: platformTenant.CreatedAt,
		UpdatedAt: platformTenant.UpdatedAt,
	}
}

func (tm *TenantMapper) ToEntity(tenant tenant.PlatformTenant) model.PlatformTenants {
	return model.PlatformTenants{
		ID:        tenant.ID,
		Name:      tenant.Name,
		Status:    string(tenant.Status),
		CreatedAt: tenant.CreatedAt,
		UpdatedAt: tenant.UpdatedAt,
	}
}

func (tm *TenantMapper) ToDomainList(entities []model.PlatformTenants) []tenant.PlatformTenant {
	domains := make([]tenant.PlatformTenant, len(entities))
	for i, entity := range entities {
		domains[i] = tm.ToDomain(entity)
	}
	return domains
}

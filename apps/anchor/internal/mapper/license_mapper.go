package mapper

import (
	"time"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/license"
)

func copyTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}

	value := *t
	return &value
}

type LicenseMapper struct{}

func NewLicenseMapper() *LicenseMapper {
	return &LicenseMapper{}
}

func (m *LicenseMapper) ToDomain(entity model.Licenses) license.License {
	return license.License{
		ID:                     entity.ID,
		ProductID:              entity.ProductID,
		OrganizationID:         entity.OrganizationID,
		PlanID:                 entity.PlanID,
		Status:                 license.Status(entity.Status),
		ExpiresAt:              copyTimePtr(entity.ExpiresAt),
		GraceUntil:             copyTimePtr(entity.GraceUntil),
		EntitlementOverrides:   unmarshalEntitlements(entity.EntitlementOverrides),
		RefreshIntervalSeconds: entity.RefreshIntervalSeconds,
		CreatedAt:              entity.CreatedAt,
		UpdatedAt:              entity.UpdatedAt,
	}
}

func (m *LicenseMapper) ToEntity(domain license.License) model.Licenses {
	return model.Licenses{
		ID:                     domain.ID,
		ProductID:              domain.ProductID,
		OrganizationID:         domain.OrganizationID,
		PlanID:                 domain.PlanID,
		Status:                 string(domain.Status),
		ExpiresAt:              copyTimePtr(domain.ExpiresAt),
		GraceUntil:             copyTimePtr(domain.GraceUntil),
		EntitlementOverrides:   marshalEntitlements(domain.EntitlementOverrides),
		RefreshIntervalSeconds: domain.RefreshIntervalSeconds,
		CreatedAt:              domain.CreatedAt,
		UpdatedAt:              domain.UpdatedAt,
	}
}

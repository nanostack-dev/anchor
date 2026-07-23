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
		ID:                   entity.ID,
		ProductID:            entity.ProductID,
		OrganizationID:       entity.OrganizationID,
		PlanID:               entity.PlanID,
		Status:               license.Status(entity.Status),
		ExpiresAt:            copyTimePtr(entity.ExpiresAt),
		GraceUntil:           copyTimePtr(entity.GraceUntil),
		EntitlementOverrides: unmarshalEntitlements(entity.EntitlementOverrides),
		TokenTTLSeconds:      entity.TokenTTLSeconds,
		CreatedAt:            entity.CreatedAt,
		UpdatedAt:            entity.UpdatedAt,
	}
}

func (m *LicenseMapper) ToEntity(domain license.License) model.Licenses {
	return model.Licenses{
		ID:                   domain.ID,
		ProductID:            domain.ProductID,
		OrganizationID:       domain.OrganizationID,
		PlanID:               domain.PlanID,
		Status:               string(domain.Status),
		ExpiresAt:            copyTimePtr(domain.ExpiresAt),
		GraceUntil:           copyTimePtr(domain.GraceUntil),
		EntitlementOverrides: marshalEntitlements(domain.EntitlementOverrides),
		TokenTTLSeconds:      domain.TokenTTLSeconds,
		CreatedAt:            domain.CreatedAt,
		UpdatedAt:            domain.UpdatedAt,
	}
}

func (m *LicenseMapper) SigningKeyToDomain(entity model.LicenseSigningKeys) license.SigningKey {
	return license.SigningKey{
		ID:                  entity.ID,
		PublicKey:           entity.PublicKey,
		PrivateKeyEncrypted: entity.PrivateKeyEncrypted,
		Status:              license.SigningKeyStatus(entity.Status),
		CreatedAt:           entity.CreatedAt,
		UpdatedAt:           entity.UpdatedAt,
	}
}

func (m *LicenseMapper) SigningKeyToEntity(domain license.SigningKey) model.LicenseSigningKeys {
	return model.LicenseSigningKeys{
		ID:                  domain.ID,
		PublicKey:           domain.PublicKey,
		PrivateKeyEncrypted: domain.PrivateKeyEncrypted,
		Status:              string(domain.Status),
		CreatedAt:           domain.CreatedAt,
		UpdatedAt:           domain.UpdatedAt,
	}
}

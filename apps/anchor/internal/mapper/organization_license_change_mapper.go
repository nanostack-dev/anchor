package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/license"
)

type OrganizationLicenseChangeMapper struct{}

func NewOrganizationLicenseChangeMapper() *OrganizationLicenseChangeMapper {
	return &OrganizationLicenseChangeMapper{}
}

func (m *OrganizationLicenseChangeMapper) ToDomain(
	entity model.OrganizationLicenseChanges,
) license.OrganizationLicenseChange {
	return license.OrganizationLicenseChange{
		ID:                 entity.ID,
		PlatformTenantID:   entity.PlatformTenantID,
		ProductID:          entity.ProductID,
		OrganizationID:     entity.OrganizationID,
		LicenseID:          entity.LicenseID,
		Type:               license.ChangeType(entity.ChangeType),
		TemplateID:         entity.TemplateID,
		PreviousTemplateID: entity.PreviousTemplateID,
		Field:              entity.Field,
		OldValue:           decodeChangeValue(entity.OldValueJSON),
		NewValue:           decodeChangeValue(entity.NewValueJSON),
		ChangedAt:          entity.ChangedAt,
	}
}

func (m *OrganizationLicenseChangeMapper) ToEntity(
	domain license.OrganizationLicenseChange,
) model.OrganizationLicenseChanges {
	return model.OrganizationLicenseChanges{
		ID:                 domain.ID,
		PlatformTenantID:   domain.PlatformTenantID,
		ProductID:          domain.ProductID,
		OrganizationID:     domain.OrganizationID,
		LicenseID:          domain.LicenseID,
		ChangeType:         string(domain.Type),
		TemplateID:         domain.TemplateID,
		PreviousTemplateID: domain.PreviousTemplateID,
		Field:              domain.Field,
		OldValueJSON:       encodeChangeValue(domain.OldValue),
		NewValueJSON:       encodeChangeValue(domain.NewValue),
		ChangedAt:          domain.ChangedAt,
	}
}

// decodeChangeValue reads a recorded value back. A decode failure means the row
// was tampered with outside the service; the entry stays readable with the
// value absent rather than failing the whole history read.
func decodeChangeValue(raw *string) any {
	if raw == nil {
		return nil
	}
	var value any
	if err := json.Unmarshal([]byte(*raw), &value); err != nil {
		return nil
	}
	return value
}

// encodeChangeValue writes a recorded value. The marshal cannot fail on a
// service-accepted value: every one passed rules.ValidateValue, which admits
// only a number, a boolean or a string, and an instantiation records a map of
// those.
func encodeChangeValue(value any) *string {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return new(string(encoded))
}

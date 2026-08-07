package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/license"
)

type OrganizationLicenseMapper struct{}

func NewOrganizationLicenseMapper() *OrganizationLicenseMapper {
	return &OrganizationLicenseMapper{}
}

func (m *OrganizationLicenseMapper) ToDomain(
	entity model.OrganizationLicenses,
) license.OrganizationLicense {
	values := license.TemplateValues{}
	if entity.ValuesJSON != "" {
		// A decode failure means the row was tampered with outside the service.
		// Degrade to an empty set so the license and its provenance stay
		// readable; the next write re-validates.
		_ = json.Unmarshal([]byte(entity.ValuesJSON), &values)
	}
	return license.OrganizationLicense{
		ID:               entity.ID,
		PlatformTenantID: entity.PlatformTenantID,
		ProductID:        entity.ProductID,
		OrganizationID:   entity.OrganizationID,
		TemplateID:       entity.TemplateID,
		InstantiatedAt:   entity.InstantiatedAt,
		Values:           values,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func (m *OrganizationLicenseMapper) ToEntity(
	domain license.OrganizationLicense,
) model.OrganizationLicenses {
	// A nil map marshals to JSON null, which the read would then special-case.
	// The marshal itself cannot fail on a service-accepted set: every value
	// passed rules.ValidateValue, which admits only number, boolean or string.
	valuesJSON := "{}"
	if domain.Values != nil {
		if b, err := json.Marshal(domain.Values); err == nil {
			valuesJSON = string(b)
		}
	}
	return model.OrganizationLicenses{
		ID:               domain.ID,
		PlatformTenantID: domain.PlatformTenantID,
		ProductID:        domain.ProductID,
		OrganizationID:   domain.OrganizationID,
		TemplateID:       domain.TemplateID,
		InstantiatedAt:   domain.InstantiatedAt,
		ValuesJSON:       valuesJSON,
		CreatedAt:        domain.CreatedAt,
		UpdatedAt:        domain.UpdatedAt,
	}
}

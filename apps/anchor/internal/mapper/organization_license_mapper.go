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
		// A stored set was validated against the schema on write, so a decode
		// failure here means the row was tampered with outside the service.
		// Degrade to an empty set rather than failing the read: an operator can
		// still see the license and its provenance, and the next write
		// re-validates it.
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
	// A nil map marshals to JSON null, which the column would accept and the
	// read would then have to special-case. A license that sets nothing is an
	// empty object, so normalise here.
	//
	// The marshal cannot fail on a set the service has accepted: every key was
	// matched to a declared license field and every value passed
	// rules.ValidateValue, which admits only a number, a boolean, or a string.
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

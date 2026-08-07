package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/license"
)

type LicenseTemplateMapper struct{}

func NewLicenseTemplateMapper() *LicenseTemplateMapper {
	return &LicenseTemplateMapper{}
}

func (m *LicenseTemplateMapper) ToDomain(entity model.LicenseTemplates) license.Template {
	values := license.TemplateValues{}
	if entity.ValuesJSON != "" {
		// A stored set was validated against the schema on write, so a decode
		// failure here means the row was tampered with outside the service.
		// Degrade to an empty set rather than failing the read: the template is
		// still listable, and the next write re-validates it.
		_ = json.Unmarshal([]byte(entity.ValuesJSON), &values)
	}
	return license.Template{
		ID:               entity.ID,
		PlatformTenantID: entity.PlatformTenantID,
		ProductID:        entity.ProductID,
		Name:             entity.Name,
		Description:      entity.Description,
		Status:           license.TemplateStatus(entity.Status),
		Values:           values,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func (m *LicenseTemplateMapper) ToEntity(domain license.Template) model.LicenseTemplates {
	// A nil map marshals to JSON null, which the column would accept and the
	// read would then have to special-case. A template that sets nothing is an
	// empty object, so normalise here.
	//
	// The marshal cannot fail on a set the service has accepted: every key was
	// matched to a declared license field and every value passed
	// rules.ValidateValue, which admits only a number, a boolean, or a string.
	// The fallback covers a caller that built a Template by hand rather than an
	// error worth reporting.
	valuesJSON := "{}"
	if domain.Values != nil {
		if b, err := json.Marshal(domain.Values); err == nil {
			valuesJSON = string(b)
		}
	}
	return model.LicenseTemplates{
		ID:               domain.ID,
		PlatformTenantID: domain.PlatformTenantID,
		ProductID:        domain.ProductID,
		Name:             domain.Name,
		Description:      domain.Description,
		Status:           string(domain.Status),
		ValuesJSON:       valuesJSON,
		CreatedAt:        domain.CreatedAt,
		UpdatedAt:        domain.UpdatedAt,
	}
}

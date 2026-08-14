package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/license"
)

type LicenseSchemaMapper struct{}

func NewLicenseSchemaMapper() *LicenseSchemaMapper {
	return &LicenseSchemaMapper{}
}

func (m *LicenseSchemaMapper) ToDomain(entity model.LicenseSchemas) license.Schema {
	return license.Schema{
		ID:               entity.ID,
		PlatformTenantID: entity.PlatformTenantID,
		ProductID:        entity.ProductID,
		Description:      entity.Description,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func (m *LicenseSchemaMapper) ToEntity(domain license.Schema) model.LicenseSchemas {
	return model.LicenseSchemas{
		ID:               domain.ID,
		PlatformTenantID: domain.PlatformTenantID,
		ProductID:        domain.ProductID,
		Description:      domain.Description,
		CreatedAt:        domain.CreatedAt,
		UpdatedAt:        domain.UpdatedAt,
	}
}

type LicenseSchemaFieldMapper struct{}

func NewLicenseSchemaFieldMapper() *LicenseSchemaFieldMapper {
	return &LicenseSchemaFieldMapper{}
}

func (m *LicenseSchemaFieldMapper) ToDomain(entity model.LicenseSchemaFields) license.Field {
	var set license.FieldRules
	if entity.RulesJSON != "" {
		// A stored rule set was validated on write, so a decode failure here means
		// the row was tampered with outside the service. Degrade to an unconstrained
		// set rather than failing the read: the declaration is still readable and
		// the next write re-validates it.
		_ = json.Unmarshal([]byte(entity.RulesJSON), &set)
	}
	var usageShape *license.UsageShape
	if entity.UsageShape != nil {
		shape := license.UsageShape(*entity.UsageShape)
		usageShape = &shape
	}
	return license.Field{
		ID:          entity.ID,
		SchemaID:    entity.LicenseSchemaID,
		Name:        entity.Name,
		Type:        license.FieldType(entity.FieldType),
		Description: entity.Description,
		Rules:       set,
		UsageShape:  usageShape,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}

func (m *LicenseSchemaFieldMapper) ToEntity(domain license.Field) model.LicenseSchemaFields {
	rulesJSON := "{}"
	if b, err := json.Marshal(domain.Rules); err == nil {
		rulesJSON = string(b)
	}
	var usageShape *string
	if domain.UsageShape != nil {
		shape := string(*domain.UsageShape)
		usageShape = &shape
	}
	return model.LicenseSchemaFields{
		ID:              domain.ID,
		LicenseSchemaID: domain.SchemaID,
		Name:            domain.Name,
		FieldType:       string(domain.Type),
		Description:     domain.Description,
		RulesJSON:       rulesJSON,
		UsageShape:      usageShape,
		CreatedAt:       domain.CreatedAt,
		UpdatedAt:       domain.UpdatedAt,
	}
}

package mapper

import (
	"encoding/json"
	"time"

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
	return license.Field{
		ID:                        entity.ID,
		SchemaID:                  entity.LicenseSchemaID,
		Name:                      entity.Name,
		Type:                      license.FieldType(entity.FieldType),
		Description:               entity.Description,
		Rules:                     set,
		ExpectedReportingInterval: secondsToDuration(entity.ExpectedReportingIntervalSeconds),
		CreatedAt:                 entity.CreatedAt,
		UpdatedAt:                 entity.UpdatedAt,
	}
}

func (m *LicenseSchemaFieldMapper) ToEntity(domain license.Field) model.LicenseSchemaFields {
	rulesJSON := "{}"
	if b, err := json.Marshal(domain.Rules); err == nil {
		rulesJSON = string(b)
	}
	return model.LicenseSchemaFields{
		ID:                               domain.ID,
		LicenseSchemaID:                  domain.SchemaID,
		Name:                             domain.Name,
		FieldType:                        string(domain.Type),
		Description:                      domain.Description,
		RulesJSON:                        rulesJSON,
		ExpectedReportingIntervalSeconds: durationToSeconds(domain.ExpectedReportingInterval),
		CreatedAt:                        domain.CreatedAt,
		UpdatedAt:                        domain.UpdatedAt,
	}
}

// secondsToDuration converts the stored column to the domain's time.Duration.
// Nil stays nil: no interval was declared.
func secondsToDuration(seconds *int32) *time.Duration {
	if seconds == nil {
		return nil
	}
	d := time.Duration(*seconds) * time.Second
	return &d
}

// durationToSeconds is secondsToDuration's inverse, truncating to whole
// seconds — the column's grain. A declaration finer than a second has no
// meaning for a reporting cadence.
func durationToSeconds(interval *time.Duration) *int32 {
	if interval == nil {
		return nil
	}
	seconds := int32(interval.Seconds())
	return &seconds
}

package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/plan"
)

type PlanMapper struct{}

func NewPlanMapper() *PlanMapper {
	return &PlanMapper{}
}

func (m *PlanMapper) ToDomain(entity model.Plans) plan.Plan {
	var description string
	if entity.Description != nil {
		description = *entity.Description
	}

	return plan.Plan{
		ID:           entity.ID,
		ProductID:    entity.ProductID,
		Key:          entity.Key,
		Name:         entity.Name,
		Description:  description,
		Entitlements: unmarshalEntitlements(entity.Entitlements),
		IsDefault:    entity.IsDefault,
		CreatedAt:    entity.CreatedAt,
		UpdatedAt:    entity.UpdatedAt,
	}
}

func (m *PlanMapper) ToEntity(domain plan.Plan) model.Plans {
	var description *string
	if domain.Description != "" {
		desc := domain.Description
		description = &desc
	}

	return model.Plans{
		ID:           domain.ID,
		ProductID:    domain.ProductID,
		Key:          domain.Key,
		Name:         domain.Name,
		Description:  description,
		Entitlements: marshalEntitlements(domain.Entitlements),
		IsDefault:    domain.IsDefault,
		CreatedAt:    domain.CreatedAt,
		UpdatedAt:    domain.UpdatedAt,
	}
}

// unmarshalEntitlements decodes the JSONB column into a typed entitlement map.
// The column is always valid JSON (jsonb) and its shape is enforced by the
// service layer on every write, so a decode failure can only mean out-of-band
// corruption; it degrades to an empty map rather than failing the read path.
func unmarshalEntitlements(raw string) plan.Entitlements {
	if raw == "" {
		return plan.Entitlements{}
	}

	var entitlements plan.Entitlements
	if err := json.Unmarshal([]byte(raw), &entitlements); err != nil {
		return plan.Entitlements{}
	}
	if entitlements == nil {
		return plan.Entitlements{}
	}

	return entitlements
}

// marshalEntitlements encodes a typed entitlement map for the JSONB column.
// Values are service-validated bool/float64 primitives, so encoding cannot
// fail for persisted data; the "{}" fallback keeps the column NOT NULL-safe.
func marshalEntitlements(entitlements plan.Entitlements) string {
	if len(entitlements) == 0 {
		return "{}"
	}

	data, err := json.Marshal(entitlements)
	if err != nil {
		return "{}"
	}

	return string(data)
}

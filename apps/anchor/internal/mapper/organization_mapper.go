package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/organization"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

type OrganizationMapper struct{}

func NewOrganizationMapper() *OrganizationMapper {
	return &OrganizationMapper{}
}

func (m *OrganizationMapper) ToDomain(entity model.Organizations) organization.Organization {
	return organization.Organization{
		ID:           entity.ID,
		ProductID:    entity.ProductID,
		Name:         entity.Name,
		Description:  entity.Description,
		MetadataJSON: MetadataJSONToDomain(entity.MetadataJSON),
		CreatedAt:    entity.CreatedAt,
		UpdatedAt:    entity.UpdatedAt,
	}
}

func (m *OrganizationMapper) ToEntity(domain organization.Organization) model.Organizations {
	return model.Organizations{
		ID:           domain.ID,
		ProductID:    domain.ProductID,
		Name:         domain.Name,
		Description:  domain.Description,
		MetadataJSON: MetadataJSONToEntity(domain.MetadataJSON),
		CreatedAt:    domain.CreatedAt,
		UpdatedAt:    domain.UpdatedAt,
	}
}

// MetadataJSONToDomain converts a nullable JSONB column value into raw JSON.
func MetadataJSONToDomain(column *string) json.RawMessage {
	return functional.FromPtr(column).Map(func(s string) json.RawMessage {
		return json.RawMessage(s)
	}).OrElse(nil)
}

// MetadataJSONToEntity converts raw JSON into a nullable JSONB column value.
// Empty metadata is stored as SQL NULL rather than an empty JSON document.
func MetadataJSONToEntity(metadata json.RawMessage) *string {
	return functional.OptionOf(metadata, len(metadata) != 0).
		Map(func(b json.RawMessage) string { return string(b) }).
		ToPtr()
}

package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/organization"
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
	if column == nil {
		return nil
	}
	return json.RawMessage(*column)
}

// MetadataJSONToEntity converts raw JSON into a nullable JSONB column value.
// Empty metadata is stored as SQL NULL rather than an empty JSON document.
func MetadataJSONToEntity(metadata json.RawMessage) *string {
	if len(metadata) == 0 {
		return nil
	}
	s := string(metadata)
	return &s
}

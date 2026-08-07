package mapper

import (
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/license"
)

type UsageObservationMapper struct{}

func NewUsageObservationMapper() *UsageObservationMapper {
	return &UsageObservationMapper{}
}

func (m *UsageObservationMapper) ToDomain(
	entity model.UsageObservations,
) license.UsageObservation {
	return license.UsageObservation{
		ID:               entity.ID,
		PlatformTenantID: entity.PlatformTenantID,
		ProductID:        entity.ProductID,
		OrganizationID:   entity.OrganizationID,
		Key:              entity.Key,
		Value:            entity.Value,
		WindowStart:      entity.WindowStart,
		WindowEnd:        entity.WindowEnd,
		ObservedAt:       entity.ObservedAt,
	}
}

func (m *UsageObservationMapper) ToEntity(
	domain license.UsageObservation,
) model.UsageObservations {
	return model.UsageObservations{
		ID:               domain.ID,
		PlatformTenantID: domain.PlatformTenantID,
		ProductID:        domain.ProductID,
		OrganizationID:   domain.OrganizationID,
		Key:              domain.Key,
		Value:            domain.Value,
		WindowStart:      domain.WindowStart,
		WindowEnd:        domain.WindowEnd,
		ObservedAt:       domain.ObservedAt,
	}
}

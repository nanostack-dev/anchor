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
		From:             entity.WindowFrom,
		To:               entity.WindowTo,
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
		WindowFrom:       domain.From,
		WindowTo:         domain.To,
		ObservedAt:       domain.ObservedAt,
	}
}

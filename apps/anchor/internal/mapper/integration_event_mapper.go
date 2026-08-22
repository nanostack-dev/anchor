package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/integration"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

type IntegrationEventMapper struct{}

func NewIntegrationEventMapper() *IntegrationEventMapper {
	return &IntegrationEventMapper{}
}

func (m *IntegrationEventMapper) ToDomain(entity model.IntegrationEvents) integration.Event {
	errStr := functional.FromPtr(entity.Error).ToPtr()

	return integration.Event{
		ID:                    entity.ID,
		IntegrationInstanceID: entity.IntegrationInstanceID,
		ExternalEventID:       entity.ExternalEventID,
		EventType:             entity.EventType,
		PayloadJSON:           json.RawMessage(entity.PayloadJSON),
		HeadersJSON:           json.RawMessage(entity.HeadersJSON),
		Status:                integration.EventStatus(entity.Status),
		Error:                 errStr,
		ProcessedAt:           entity.ProcessedAt,
		CreatedAt:             entity.CreatedAt,
		UpdatedAt:             entity.UpdatedAt,
	}
}

func (m *IntegrationEventMapper) ToEntity(domain integration.Event) model.IntegrationEvents {
	errStr := functional.FromPtr(domain.Error).ToPtr()

	payloadStr := "{}"
	if domain.PayloadJSON != nil {
		payloadStr = string(domain.PayloadJSON)
	}

	headersStr := "{}"
	if domain.HeadersJSON != nil {
		headersStr = string(domain.HeadersJSON)
	}

	return model.IntegrationEvents{
		ID:                    domain.ID,
		IntegrationInstanceID: domain.IntegrationInstanceID,
		ExternalEventID:       domain.ExternalEventID,
		EventType:             domain.EventType,
		PayloadJSON:           payloadStr,
		HeadersJSON:           headersStr,
		Status:                string(domain.Status),
		Error:                 errStr,
		ProcessedAt:           domain.ProcessedAt,
		CreatedAt:             domain.CreatedAt,
		UpdatedAt:             domain.UpdatedAt,
	}
}

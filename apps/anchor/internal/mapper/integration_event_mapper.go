package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/integration"
)

type IntegrationEventMapper struct{}

func NewIntegrationEventMapper() *IntegrationEventMapper {
	return &IntegrationEventMapper{}
}

func (m *IntegrationEventMapper) ToDomain(entity model.IntegrationEvents) integration.Event {
	var errStr *string
	if entity.Error != nil {
		e := *entity.Error
		errStr = &e
	}

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
	var errStr *string
	if domain.Error != nil {
		e := *domain.Error
		errStr = &e
	}

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

package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/integration"
)

type IntegrationInstanceMapper struct{}

func NewIntegrationInstanceMapper() *IntegrationInstanceMapper {
	return &IntegrationInstanceMapper{}
}

func (m *IntegrationInstanceMapper) ToDomain(entity model.IntegrationInstances) integration.Instance {
	var webhookSecret *string
	if entity.WebhookSecret != nil {
		ws := *entity.WebhookSecret
		webhookSecret = &ws
	}

	var lastError *string
	if entity.LastError != nil {
		le := *entity.LastError
		lastError = &le
	}

	return integration.Instance{
		ID:               entity.ID,
		PlatformTenantID: entity.PlatformTenantID,
		ProductID:        entity.ProductID,
		ProviderType:     integration.ProviderType(entity.ProviderType),
		WebhookSecret:    webhookSecret,
		ConfigJSON:       json.RawMessage(entity.ConfigJSON),
		ConfigVersion:    entity.ConfigVersion,
		IsEnabled:        entity.IsEnabled,
		LastError:        lastError,
		Status:           integration.Status(entity.Status),
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func (m *IntegrationInstanceMapper) ToEntity(domain integration.Instance) model.IntegrationInstances {
	var webhookSecret *string
	if domain.WebhookSecret != nil {
		ws := *domain.WebhookSecret
		webhookSecret = &ws
	}

	var lastError *string
	if domain.LastError != nil {
		le := *domain.LastError
		lastError = &le
	}

	configStr := "{}"
	if domain.ConfigJSON != nil {
		configStr = string(domain.ConfigJSON)
	}

	return model.IntegrationInstances{
		ID:               domain.ID,
		PlatformTenantID: domain.PlatformTenantID,
		ProductID:        domain.ProductID,
		ProviderType:     string(domain.ProviderType),
		WebhookSecret:    webhookSecret,
		ConfigJSON:       configStr,
		ConfigVersion:    domain.ConfigVersion,
		IsEnabled:        domain.IsEnabled,
		LastError:        lastError,
		Status:           string(domain.Status),
		CreatedAt:        domain.CreatedAt,
		UpdatedAt:        domain.UpdatedAt,
	}
}

package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/integration"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

type IntegrationInstanceMapper struct{}

func NewIntegrationInstanceMapper() *IntegrationInstanceMapper {
	return &IntegrationInstanceMapper{}
}

func (m *IntegrationInstanceMapper) ToDomain(entity model.IntegrationInstances) integration.Instance {
	webhookSecret := functional.FromPtr(entity.WebhookSecret).ToPtr()
	lastError := functional.FromPtr(entity.LastError).ToPtr()

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
	webhookSecret := functional.FromPtr(domain.WebhookSecret).ToPtr()
	lastError := functional.FromPtr(domain.LastError).ToPtr()

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

package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/integration"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

type IntegrationAuditLogMapper struct{}

func NewIntegrationAuditLogMapper() *IntegrationAuditLogMapper {
	return &IntegrationAuditLogMapper{}
}

func (m *IntegrationAuditLogMapper) ToDomain(entity model.IntegrationAuditLogs) integration.AuditLog {
	externalID := functional.FromPtr(entity.ExternalID).ToPtr()
	internalID := functional.FromPtr(entity.InternalID).ToPtr()

	diffJSON := functional.FromPtr(entity.DiffJSON).Map(func(s string) json.RawMessage {
		return json.RawMessage(s)
	}).OrElse(nil)

	metadataJSON := functional.FromPtr(entity.MetadataJSON).Map(func(s string) json.RawMessage {
		return json.RawMessage(s)
	}).OrElse(nil)

	return integration.AuditLog{
		ID:                    entity.ID,
		IntegrationInstanceID: entity.IntegrationInstanceID,
		Action:                entity.Action,
		Severity:              integration.AuditSeverity(entity.Severity),
		Message:               entity.Message,
		MetadataJSON:          metadataJSON,
		EntityType:            entity.EntityType,
		ExternalID:            externalID,
		InternalID:            internalID,
		DiffJSON:              diffJSON,
		CreatedAt:             entity.CreatedAt,
	}
}

func (m *IntegrationAuditLogMapper) ToEntity(domain integration.AuditLog) model.IntegrationAuditLogs {
	externalID := functional.FromPtr(domain.ExternalID).ToPtr()
	internalID := functional.FromPtr(domain.InternalID).ToPtr()

	diffStr := functional.OptionOf(domain.DiffJSON, domain.DiffJSON != nil).
		Map(func(b json.RawMessage) string { return string(b) }).
		ToPtr()

	metadataStr := functional.OptionOf(domain.MetadataJSON, domain.MetadataJSON != nil).
		Map(func(b json.RawMessage) string { return string(b) }).
		ToPtr()

	return model.IntegrationAuditLogs{
		ID:                    domain.ID,
		IntegrationInstanceID: domain.IntegrationInstanceID,
		Action:                domain.Action,
		Severity:              string(domain.Severity),
		Message:               domain.Message,
		MetadataJSON:          metadataStr,
		EntityType:            domain.EntityType,
		ExternalID:            externalID,
		InternalID:            internalID,
		DiffJSON:              diffStr,
		CreatedAt:             domain.CreatedAt,
	}
}

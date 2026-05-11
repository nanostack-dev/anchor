package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/integration"
)

type IntegrationAuditLogMapper struct{}

func NewIntegrationAuditLogMapper() *IntegrationAuditLogMapper {
	return &IntegrationAuditLogMapper{}
}

func (m *IntegrationAuditLogMapper) ToDomain(entity model.IntegrationAuditLogs) integration.AuditLog {
	var externalID, internalID *string
	if entity.ExternalID != nil {
		e := *entity.ExternalID
		externalID = &e
	}
	if entity.InternalID != nil {
		i := *entity.InternalID
		internalID = &i
	}

	var diffJSON json.RawMessage
	if entity.DiffJSON != nil {
		diffJSON = json.RawMessage(*entity.DiffJSON)
	}

	var metadataJSON json.RawMessage
	if entity.MetadataJSON != nil {
		metadataJSON = json.RawMessage(*entity.MetadataJSON)
	}

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
	var externalID, internalID *string
	if domain.ExternalID != nil {
		e := *domain.ExternalID
		externalID = &e
	}
	if domain.InternalID != nil {
		i := *domain.InternalID
		internalID = &i
	}

	var diffStr *string
	if domain.DiffJSON != nil {
		s := string(domain.DiffJSON)
		diffStr = &s
	}

	var metadataStr *string
	if domain.MetadataJSON != nil {
		s := string(domain.MetadataJSON)
		metadataStr = &s
	}

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

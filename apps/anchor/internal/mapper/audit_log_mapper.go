package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/audit"
)

type AuditLogMapper struct{}

func NewAuditLogMapper() *AuditLogMapper {
	return &AuditLogMapper{}
}

func (m *AuditLogMapper) ToDomain(entity model.AuditLogs) audit.Log {
	var metadataJSON json.RawMessage
	if entity.MetadataJSON != nil {
		metadataJSON = json.RawMessage(*entity.MetadataJSON)
	}

	return audit.Log{
		ID:               entity.ID,
		PlatformTenantID: entity.PlatformTenantID,
		ProductID:        entity.ProductID,
		OrganizationID:   copyOptionalString(entity.OrganizationID),
		Action:           audit.Action(entity.Action),
		Outcome:          audit.Outcome(entity.Outcome),
		ActorType:        audit.ActorType(entity.ActorType),
		ActorID:          copyOptionalString(entity.ActorID),
		ActorName:        copyOptionalString(entity.ActorName),
		TargetType:       entity.TargetType,
		TargetID:         copyOptionalString(entity.TargetID),
		TargetName:       copyOptionalString(entity.TargetName),
		RequestID:        copyOptionalString(entity.RequestID),
		MetadataJSON:     metadataJSON,
		CreatedAt:        entity.CreatedAt,
	}
}

func (m *AuditLogMapper) ToEntity(domain audit.Log) model.AuditLogs {
	var metadataStr *string
	if domain.MetadataJSON != nil {
		s := string(domain.MetadataJSON)
		metadataStr = &s
	}

	return model.AuditLogs{
		ID:               domain.ID,
		PlatformTenantID: domain.PlatformTenantID,
		ProductID:        domain.ProductID,
		OrganizationID:   copyOptionalString(domain.OrganizationID),
		Action:           string(domain.Action),
		Outcome:          string(domain.Outcome),
		ActorType:        string(domain.ActorType),
		ActorID:          copyOptionalString(domain.ActorID),
		ActorName:        copyOptionalString(domain.ActorName),
		TargetType:       domain.TargetType,
		TargetID:         copyOptionalString(domain.TargetID),
		TargetName:       copyOptionalString(domain.TargetName),
		RequestID:        copyOptionalString(domain.RequestID),
		MetadataJSON:     metadataStr,
		CreatedAt:        domain.CreatedAt,
	}
}

func copyOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

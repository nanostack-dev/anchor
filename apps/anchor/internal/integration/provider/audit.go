package provider

import (
	"context"
	"encoding/json"
	"time"

	"anchor/internal/domain/integration"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
)

// WriteAuditLog persists an integration audit log entry, filling in defaults
// for missing fields. It is a shared helper used by command handlers across
// all providers.
func WriteAuditLog(
	ctx context.Context,
	logger zerolog.Logger,
	repo repository.IntegrationAuditLogRepository,
	auditLog integration.AuditLog,
) {
	if auditLog.ID == "" {
		auditLog.GenerateID()
	}
	if auditLog.CreatedAt.IsZero() {
		auditLog.CreatedAt = time.Now()
	}
	if auditLog.Severity == "" {
		auditLog.Severity = integration.AuditSeverityInfo
	}
	if auditLog.Message == "" {
		auditLog.Message = "Integration activity recorded"
	}

	if _, auditErr := repo.Create(ctx, auditLog); auditErr != nil {
		logger.Error().Err(auditErr).
			Str("integration_instance_id", auditLog.IntegrationInstanceID).
			Str("action", auditLog.Action).
			Msg("failed to create integration audit log")
	}
}

// MustMarshalJSON marshals a value to JSON, returning an empty object on failure.
func MustMarshalJSON(value any) json.RawMessage {
	raw, marshalErr := json.Marshal(value)
	if marshalErr != nil {
		return json.RawMessage("{}")
	}
	return raw
}

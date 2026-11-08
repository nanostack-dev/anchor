package provider

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"anchor/internal/domain/integration"
	"anchor/internal/repository"
)

// WriteAuditLog persists an integration audit log entry, filling in defaults
// for missing fields. It is a shared helper used by command handlers across
// all providers.
func WriteAuditLog(
	ctx context.Context,
	logger *slog.Logger,
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
		logger.ErrorContext(ctx, "failed to create integration audit log",
			log.Err(auditErr),
			slog.String("integration_instance_id", auditLog.IntegrationInstanceID),
			slog.String("action", auditLog.Action),
		)
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

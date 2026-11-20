package provider

import (
	"context"
	"encoding/json"
	"errors"
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

// FailureAuditError carries the audit row that records a failed command.
//
// A command handler runs inside its caller's transaction. When its write fails,
// PostgreSQL aborts that transaction, so an audit row written there fails with
// 25P02 and would roll back anyway. The handler returns this error instead, and
// the caller writes the row in a fresh transaction after the rollback.
type FailureAuditError struct {
	Audit integration.AuditLog
	Err   error
}

func (e *FailureAuditError) Error() string { return e.Err.Error() }

func (e *FailureAuditError) Unwrap() error { return e.Err }

// WithFailureAudit attaches the audit row that records err. Write failure audit
// rows this way, never with WriteAuditLog on the context of the failed work.
func WithFailureAudit(err error, auditLog integration.AuditLog) error {
	return &FailureAuditError{Audit: auditLog, Err: err}
}

// FailureAudit returns the audit row attached to err by WithFailureAudit.
func FailureAudit(err error) (integration.AuditLog, bool) {
	if auditErr, ok := errors.AsType[*FailureAuditError](err); ok {
		return auditErr.Audit, true
	}
	return integration.AuditLog{}, false
}

// MustMarshalJSON marshals a value to JSON, returning an empty object on failure.
func MustMarshalJSON(value any) json.RawMessage {
	raw, marshalErr := json.Marshal(value)
	if marshalErr != nil {
		return json.RawMessage("{}")
	}
	return raw
}

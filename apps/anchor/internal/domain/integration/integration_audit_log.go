package integration

import (
	"encoding/json"
	"time"

	"github.com/nanostack-dev/shared/toolkit"
)

type AuditLog struct {
	ID                    string
	IntegrationInstanceID string
	Action                string
	Severity              AuditSeverity
	Message               string
	MetadataJSON          json.RawMessage
	EntityType            string
	ExternalID            *string
	InternalID            *string
	DiffJSON              json.RawMessage
	CreatedAt             time.Time
}

// GenerateID sets the audit log's ID to a new prefixed KSUID.
func (a *AuditLog) GenerateID() {
	a.ID = toolkit.NewID("ial")
}

type AuditSeverity string

const (
	AuditSeverityInfo    AuditSeverity = "INFO"
	AuditSeveritySuccess AuditSeverity = "SUCCESS"
	AuditSeverityWarning AuditSeverity = "WARNING"
	AuditSeverityError   AuditSeverity = "ERROR"
)

const (
	AuditActionIntegrationCreated  = "INTEGRATION_CREATED"
	AuditActionConfigUpdated       = "CONFIG_UPDATED"
	AuditActionIntegrationDeleted  = "INTEGRATION_DELETED"
	AuditActionWebhookReceived     = "WEBHOOK_RECEIVED"
	AuditActionWebhookProcessed    = "WEBHOOK_PROCESSED"
	AuditActionWebhookFailed       = "WEBHOOK_PROCESSING_FAILED"
	AuditActionSignatureInvalid    = "SIGNATURE_INVALID"
	AuditActionUpsertUser          = "UPSERT_USER"
	AuditActionDeleteUser          = "DELETE_USER"
	AuditActionCommandTypeInvalid  = "COMMAND_TYPE_INVALID"
	AuditActionEventParseFailed    = "EVENT_PARSE_FAILED"
	AuditActionCommandsBuildFailed = "COMMAND_BUILD_FAILED"
	AuditActionEventRetryScheduled = "EVENT_RETRY_SCHEDULED"
	AuditActionReconcileCompleted  = "RECONCILE_COMPLETED"
	AuditActionReconcileBatch      = "RECONCILE_BATCH"
)

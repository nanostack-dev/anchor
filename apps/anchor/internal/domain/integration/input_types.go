package integration

import (
	"encoding/json"
	"time"

	"anchor/internal/searchx"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

type CreateInstanceInput struct {
	TenantID     string       `validate:"required"`
	ProductID    string       `validate:"required"`
	ProviderType ProviderType `validate:"required"`
	ConfigJSON   json.RawMessage
}

type GetInstanceInput struct {
	TenantID  string `validate:"required"`
	ProductID string `validate:"required"`
	ID        string `validate:"required"`
}

type UpdateInstanceInput struct {
	TenantID      string `validate:"required"`
	ProductID     string `validate:"required"`
	ID            string `validate:"required"`
	WebhookSecret *string
	ConfigJSON    json.RawMessage
	IsEnabled     *bool
}

type DeleteInstanceInput struct {
	TenantID  string `validate:"required"`
	ProductID string `validate:"required"`
	ID        string `validate:"required"`
}

type ListInstancesInput struct {
	TenantID  string `validate:"required"`
	ProductID string `validate:"required"`
}

type IngestWebhookInput struct {
	TenantID     string
	ProductID    string
	ProviderType ProviderType
	Payload      []byte
	Headers      map[string]string
}

type CreateAuditLogInput struct {
	IntegrationInstanceID string
	Action                string
	EntityType            string
	ExternalID            *string
	InternalID            *string
	DiffJSON              json.RawMessage
}

// SortFieldAuditLog names a column an audit log search may order by.
type SortFieldAuditLog string

const (
	SortFieldAuditLogCreatedAt SortFieldAuditLog = "created_at"
	SortFieldAuditLogAction    SortFieldAuditLog = "action"
	SortFieldAuditLogSeverity  SortFieldAuditLog = "severity"
)

// SearchAuditLogsFilter narrows an audit log search. Empty fields do not
// filter.
type SearchAuditLogsFilter struct {
	Actions       []string
	Severities    []AuditSeverity `validate:"omitempty,dive,oneof=INFO SUCCESS WARNING ERROR"`
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
}

type SearchAuditLogsInput struct {
	TenantID              string                                                   `validate:"required"`
	ProductID             string                                                   `validate:"required"`
	IntegrationInstanceID string                                                   `validate:"required"`
	Request               search.Request[SearchAuditLogsFilter, SortFieldAuditLog] `validate:"required"`
}

// Validate checks the input, the sort the client named and the actions it filters on.
func (in SearchAuditLogsInput) Validate() error {
	if err := searchx.ValidateRequest(in, in.Request,
		SortFieldAuditLogCreatedAt,
		SortFieldAuditLogAction,
		SortFieldAuditLogSeverity); err != nil {
		return err
	}
	if in.Request.Filter == nil {
		return nil
	}
	return searchx.ValidateOneOf("filter.actions", in.Request.Filter.Actions, AuditActions())
}

package integration

import (
	"encoding/json"
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

// SortFieldAuditLog is the sort_by type of POST /integrations/{id}/audit-logs/search.
// The generated search owns the values; this type gives the OpenAPI schema its Go type.
type SortFieldAuditLog string

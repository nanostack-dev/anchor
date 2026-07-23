package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/webhook"
)

// ---------------------------------------------------------------------------
// Endpoints
// ---------------------------------------------------------------------------

type WebhookEndpointMapper struct{}

func NewWebhookEndpointMapper() *WebhookEndpointMapper {
	return &WebhookEndpointMapper{}
}

func (m *WebhookEndpointMapper) ToDomain(entity model.WebhookEndpoints) webhook.Endpoint {
	return webhook.Endpoint{
		ID:                      entity.ID,
		ProductID:               entity.ProductID,
		URL:                     entity.URL,
		Description:             stringOrEmpty(entity.Description),
		EventTypes:              unmarshalEventTypes(entity.EventTypes),
		Status:                  webhook.EndpointStatus(entity.Status),
		DisabledReason:          stringOrEmpty(entity.DisabledReason),
		ConsecutiveFailureCount: entity.ConsecutiveFailureCount,
		FirstFailureAt:          copyTimePtr(entity.FirstFailureAt),
		LastFailureAt:           copyTimePtr(entity.LastFailureAt),
		LastSuccessAt:           copyTimePtr(entity.LastSuccessAt),
		CreatedAt:               entity.CreatedAt,
		UpdatedAt:               entity.UpdatedAt,
	}
}

func (m *WebhookEndpointMapper) ToEntity(domain webhook.Endpoint) model.WebhookEndpoints {
	return model.WebhookEndpoints{
		ID:                      domain.ID,
		ProductID:               domain.ProductID,
		URL:                     domain.URL,
		Description:             nilIfEmpty(domain.Description),
		EventTypes:              marshalEventTypes(domain.EventTypes),
		Status:                  string(domain.Status),
		DisabledReason:          nilIfEmpty(domain.DisabledReason),
		ConsecutiveFailureCount: domain.ConsecutiveFailureCount,
		FirstFailureAt:          copyTimePtr(domain.FirstFailureAt),
		LastFailureAt:           copyTimePtr(domain.LastFailureAt),
		LastSuccessAt:           copyTimePtr(domain.LastSuccessAt),
		CreatedAt:               domain.CreatedAt,
		UpdatedAt:               domain.UpdatedAt,
	}
}

// ---------------------------------------------------------------------------
// Endpoint secrets
// ---------------------------------------------------------------------------

type WebhookEndpointSecretMapper struct{}

func NewWebhookEndpointSecretMapper() *WebhookEndpointSecretMapper {
	return &WebhookEndpointSecretMapper{}
}

func (m *WebhookEndpointSecretMapper) ToDomain(
	entity model.WebhookEndpointSecrets,
) webhook.Secret {
	return webhook.Secret{
		ID:              entity.ID,
		EndpointID:      entity.EndpointID,
		EncryptedSecret: entity.EncryptedSecret,
		Status:          webhook.SecretStatus(entity.Status),
		ExpiresAt:       copyTimePtr(entity.ExpiresAt),
		CreatedAt:       entity.CreatedAt,
		UpdatedAt:       entity.UpdatedAt,
	}
}

func (m *WebhookEndpointSecretMapper) ToEntity(
	domain webhook.Secret,
) model.WebhookEndpointSecrets {
	return model.WebhookEndpointSecrets{
		ID:              domain.ID,
		EndpointID:      domain.EndpointID,
		EncryptedSecret: domain.EncryptedSecret,
		Status:          string(domain.Status),
		ExpiresAt:       copyTimePtr(domain.ExpiresAt),
		CreatedAt:       domain.CreatedAt,
		UpdatedAt:       domain.UpdatedAt,
	}
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

type WebhookEventMapper struct{}

func NewWebhookEventMapper() *WebhookEventMapper {
	return &WebhookEventMapper{}
}

func (m *WebhookEventMapper) ToDomain(entity model.WebhookEvents) webhook.Event {
	payload := json.RawMessage(entity.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	return webhook.Event{
		ID:               entity.ID,
		ProductID:        entity.ProductID,
		OrganizationID:   copyStringPtr(entity.OrganizationID),
		EventType:        entity.EventType,
		APIVersion:       entity.APIVersion,
		Payload:          payload,
		OccurredAt:       entity.OccurredAt,
		TargetEndpointID: copyStringPtr(entity.TargetEndpointID),
		CreatedAt:        entity.CreatedAt,
	}
}

func (m *WebhookEventMapper) ToEntity(domain webhook.Event) model.WebhookEvents {
	payload := string(domain.Payload)
	if payload == "" {
		payload = "{}"
	}

	return model.WebhookEvents{
		ID:               domain.ID,
		ProductID:        domain.ProductID,
		OrganizationID:   copyStringPtr(domain.OrganizationID),
		EventType:        domain.EventType,
		APIVersion:       domain.APIVersion,
		Payload:          payload,
		OccurredAt:       domain.OccurredAt,
		TargetEndpointID: copyStringPtr(domain.TargetEndpointID),
		CreatedAt:        domain.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// Deliveries and attempts
// ---------------------------------------------------------------------------

type WebhookDeliveryMapper struct{}

func NewWebhookDeliveryMapper() *WebhookDeliveryMapper {
	return &WebhookDeliveryMapper{}
}

func (m *WebhookDeliveryMapper) ToDomain(entity model.WebhookDeliveries) webhook.Delivery {
	return webhook.Delivery{
		ID:                 entity.ID,
		EventID:            entity.EventID,
		EndpointID:         entity.EndpointID,
		ProductID:          entity.ProductID,
		Status:             webhook.DeliveryStatus(entity.Status),
		AttemptCount:       entity.AttemptCount,
		MaxAttempts:        entity.MaxAttempts,
		TargetURL:          entity.TargetURL,
		SignedBody:         entity.SignedBody,
		LastStatusCode:     copyInt32Ptr(entity.LastStatusCode),
		LastError:          copyStringPtr(entity.LastError),
		CompletedAt:        copyTimePtr(entity.CompletedAt),
		ReplayOfDeliveryID: copyStringPtr(entity.ReplayOfDeliveryID),
		CreatedAt:          entity.CreatedAt,
		UpdatedAt:          entity.UpdatedAt,
	}
}

func (m *WebhookDeliveryMapper) ToEntity(domain webhook.Delivery) model.WebhookDeliveries {
	return model.WebhookDeliveries{
		ID:                 domain.ID,
		EventID:            domain.EventID,
		EndpointID:         domain.EndpointID,
		ProductID:          domain.ProductID,
		Status:             string(domain.Status),
		AttemptCount:       domain.AttemptCount,
		MaxAttempts:        domain.MaxAttempts,
		TargetURL:          domain.TargetURL,
		SignedBody:         domain.SignedBody,
		LastStatusCode:     copyInt32Ptr(domain.LastStatusCode),
		LastError:          copyStringPtr(domain.LastError),
		CompletedAt:        copyTimePtr(domain.CompletedAt),
		ReplayOfDeliveryID: copyStringPtr(domain.ReplayOfDeliveryID),
		CreatedAt:          domain.CreatedAt,
		UpdatedAt:          domain.UpdatedAt,
	}
}

type WebhookDeliveryAttemptMapper struct{}

func NewWebhookDeliveryAttemptMapper() *WebhookDeliveryAttemptMapper {
	return &WebhookDeliveryAttemptMapper{}
}

func (m *WebhookDeliveryAttemptMapper) ToDomain(
	entity model.WebhookDeliveryAttempts,
) webhook.Attempt {
	return webhook.Attempt{
		ID:              entity.ID,
		DeliveryID:      entity.DeliveryID,
		AttemptNumber:   entity.AttemptNumber,
		StatusCode:      copyInt32Ptr(entity.StatusCode),
		Error:           copyStringPtr(entity.Error),
		ResponseSnippet: copyStringPtr(entity.ResponseSnippet),
		DurationMs:      entity.DurationMs,
		AttemptedAt:     entity.AttemptedAt,
		CreatedAt:       entity.CreatedAt,
	}
}

func (m *WebhookDeliveryAttemptMapper) ToEntity(
	domain webhook.Attempt,
) model.WebhookDeliveryAttempts {
	return model.WebhookDeliveryAttempts{
		ID:              domain.ID,
		DeliveryID:      domain.DeliveryID,
		AttemptNumber:   domain.AttemptNumber,
		StatusCode:      copyInt32Ptr(domain.StatusCode),
		Error:           copyStringPtr(domain.Error),
		ResponseSnippet: copyStringPtr(domain.ResponseSnippet),
		DurationMs:      domain.DurationMs,
		AttemptedAt:     domain.AttemptedAt,
		CreatedAt:       domain.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// unmarshalEventTypes decodes the JSONB subscription array. The column is
// written only by the service layer after registry validation, so a decode
// failure can only mean out-of-band corruption; it degrades to "subscribed to
// nothing" rather than failing the read path.
func unmarshalEventTypes(raw string) []string {
	if raw == "" {
		return []string{}
	}

	var eventTypes []string
	if err := json.Unmarshal([]byte(raw), &eventTypes); err != nil {
		return []string{}
	}
	if eventTypes == nil {
		return []string{}
	}

	return eventTypes
}

// marshalEventTypes encodes the subscription array for the JSONB column. The
// "[]" fallback keeps the column NOT NULL-safe.
func marshalEventTypes(eventTypes []string) string {
	if len(eventTypes) == 0 {
		return "[]"
	}

	data, err := json.Marshal(eventTypes)
	if err != nil {
		return "[]"
	}

	return string(data)
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}

	copied := value
	return &copied
}

func copyStringPtr(value *string) *string {
	if value == nil {
		return nil
	}

	copied := *value
	return &copied
}

func copyInt32Ptr(value *int32) *int32 {
	if value == nil {
		return nil
	}

	copied := *value
	return &copied
}

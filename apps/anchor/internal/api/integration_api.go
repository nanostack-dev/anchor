package api

import (
	"context"
	"encoding/json"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"

	"anchor/internal/domain/integration"
	"anchor/internal/integration/provider/smtp"
	"anchor/internal/security"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mapInstanceToResponse(inst integration.Instance) IntegrationInstanceResponse {
	webhookSecretObfuscated := obfuscateSecret(inst.WebhookSecret)

	resp := IntegrationInstanceResponse{
		Id:                      inst.ID,
		ProductId:               inst.ProductID,
		ProviderType:            inst.ProviderType,
		ConfigVersion:           inst.ConfigVersion,
		IsEnabled:               inst.IsEnabled,
		Status:                  inst.Status,
		LastError:               inst.LastError,
		WebhookSecretObfuscated: webhookSecretObfuscated,
		CreatedAt:               inst.CreatedAt,
		UpdatedAt:               inst.UpdatedAt,
	}

	if pc := buildPublicConfig(inst); pc != nil {
		resp.PublicConfig = pc
	}

	return resp
}

func buildPublicConfig(inst integration.Instance) *IntegrationProviderPublicConfig {
	if len(inst.ConfigJSON) == 0 {
		return nil
	}

	switch inst.ProviderType {
	case integration.ProviderTypeClerk:
		return nil
	case integration.ProviderTypeSMTP:
		var cfg smtp.Config
		if err := json.Unmarshal(inst.ConfigJSON, &cfg); err != nil {
			return nil
		}
		enc := SmtpIntegrationPublicConfigEncryption(cfg.Encryption)
		auth := SmtpIntegrationPublicConfigAuthMethod(cfg.AuthMethod)
		port := cfg.Port
		pub := SmtpIntegrationPublicConfig{
			Host:        &cfg.Host,
			Port:        &port,
			Encryption:  &enc,
			AuthMethod:  &auth,
			Username:    &cfg.Username,
			FromAddress: &cfg.FromAddress,
		}
		if cfg.FromName != "" {
			pub.FromName = &cfg.FromName
		}
		if cfg.ReplyTo != "" {
			pub.ReplyTo = &cfg.ReplyTo
		}
		wrapper := IntegrationProviderPublicConfig{}
		if err := wrapper.FromSmtpIntegrationPublicConfig(pub); err != nil {
			return nil
		}
		return &wrapper
	default:
		return nil
	}
}

func obfuscateSecret(secret *string) *string {
	if secret == nil || *secret == "" {
		return nil
	}

	masked := secrets.Obfuscate(*secret)
	return &masked
}

func mapAuditLogToResponse(log integration.AuditLog) IntegrationAuditLogEntryResponse {
	var metadata *map[string]interface{}
	if len(log.MetadataJSON) > 0 {
		parsedMetadata := map[string]interface{}{}
		if err := json.Unmarshal(log.MetadataJSON, &parsedMetadata); err == nil {
			metadata = &parsedMetadata
		}
	}

	return IntegrationAuditLogEntryResponse{
		Id:                    log.ID,
		IntegrationInstanceId: log.IntegrationInstanceID,
		Action:                log.Action,
		Severity:              log.Severity,
		Message:               log.Message,
		Metadata:              metadata,
		CreatedAt:             log.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// Instance CRUD
// ---------------------------------------------------------------------------

func (s *AnchorAPI) CreateIntegrationInstance(
	ctx context.Context, request CreateIntegrationInstanceRequestObject,
) (CreateIntegrationInstanceResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	var configJSON json.RawMessage
	var providerType integration.ProviderType

	discriminator, err := request.Body.Discriminator()
	if err != nil {
		return nil, err
	}

	switch discriminator {
	case string(integration.ProviderTypeClerk):
		providerType = integration.ProviderTypeClerk
		payload, parseErr := request.Body.AsClerkIntegrationInstanceCreateRequest()
		if parseErr != nil {
			return nil, parseErr
		}
		if payload.Config != nil {
			configJSON, err = json.Marshal(
				payload.Config,
			) // #nosec G117 -- raw config is passed to service-layer encryption before storage
			if err != nil {
				return nil, err
			}
		}
	case string(integration.ProviderTypeSMTP):
		providerType = integration.ProviderTypeSMTP
		payload, parseErr := request.Body.AsSmtpIntegrationInstanceCreateRequest()
		if parseErr != nil {
			return nil, parseErr
		}
		configJSON, err = json.Marshal(
			payload.Config,
		) // #nosec G117 -- raw config is passed to service-layer encryption before storage
		if err != nil {
			return nil, err
		}
	default:
		return nil, fault.BadRequest(
			"INTEGRATION_PROVIDER_INVALID",
			"Unsupported integration provider type",
		)
	}

	input := integration.CreateInstanceInput{
		TenantID:     tenantID,
		ProductID:    request.ProductId,
		ProviderType: providerType,
		ConfigJSON:   configJSON,
	}

	created, err := s.IntegrationService.CreateInstance(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Msg("failed to create integration instance")
		return nil, err
	}

	return CreateIntegrationInstance201JSONResponse(mapInstanceToResponse(created)), nil
}

func (s *AnchorAPI) ListIntegrationInstances(
	ctx context.Context, request ListIntegrationInstancesRequestObject,
) (ListIntegrationInstancesResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	input := integration.ListInstancesInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
	}

	instances, err := s.IntegrationService.ListInstances(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Msg("failed to list integration instances")
		return nil, err
	}

	items := make([]IntegrationInstanceResponse, 0, len(instances))
	for _, inst := range instances {
		items = append(items, mapInstanceToResponse(inst))
	}

	return ListIntegrationInstances200JSONResponse(IntegrationInstanceListResponse{
		Items: items,
		Count: len(items),
	}), nil
}

func (s *AnchorAPI) GetIntegrationInstance(
	ctx context.Context, request GetIntegrationInstanceRequestObject,
) (GetIntegrationInstanceResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	input := integration.GetInstanceInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		ID:        request.IntegrationInstanceId,
	}

	instance, err := s.IntegrationService.GetInstance(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("integration_instance_id", request.IntegrationInstanceId).
			Msg("failed to get integration instance")
		return nil, err
	}

	if instance == nil {
		return GetIntegrationInstance404Response{}, nil
	}

	return GetIntegrationInstance200JSONResponse(mapInstanceToResponse(*instance)), nil
}

func (s *AnchorAPI) UpdateIntegrationInstance(
	ctx context.Context, request UpdateIntegrationInstanceRequestObject,
) (UpdateIntegrationInstanceResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	input := integration.UpdateInstanceInput{
		TenantID:      tenantID,
		ProductID:     request.ProductId,
		ID:            request.IntegrationInstanceId,
		WebhookSecret: request.Body.WebhookSecret,
	}

	if request.Body.IsEnabled != nil {
		isEnabled := *request.Body.IsEnabled
		input.IsEnabled = &isEnabled
	}

	if request.Body.Config != nil {
		configJSON, marshalErr := json.Marshal(request.Body.Config)
		if marshalErr != nil {
			return nil, marshalErr
		}
		input.ConfigJSON = configJSON
	}

	updated, err := s.IntegrationService.UpdateInstance(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("integration_instance_id", request.IntegrationInstanceId).
			Msg("failed to update integration instance")
		return nil, err
	}

	return UpdateIntegrationInstance200JSONResponse(mapInstanceToResponse(updated)), nil
}

func (s *AnchorAPI) DeleteIntegrationInstance(
	ctx context.Context, request DeleteIntegrationInstanceRequestObject,
) (DeleteIntegrationInstanceResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	input := integration.DeleteInstanceInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		ID:        request.IntegrationInstanceId,
	}

	err = s.IntegrationService.DeleteInstance(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("integration_instance_id", request.IntegrationInstanceId).
			Msg("failed to delete integration instance")
		return nil, err
	}

	return DeleteIntegrationInstance204Response{}, nil
}

func (s *AnchorAPI) ListIntegrationAuditLogs(
	ctx context.Context, request ListIntegrationAuditLogsRequestObject,
) (ListIntegrationAuditLogsResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	input := integration.ListAuditLogsInput{
		TenantID:              tenantID,
		ProductID:             request.ProductId,
		IntegrationInstanceID: request.IntegrationInstanceId,
	}

	logs, err := s.IntegrationService.ListAuditLogs(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("integration_instance_id", request.IntegrationInstanceId).
			Msg("failed to list integration audit logs")
		return nil, err
	}

	items := make([]IntegrationAuditLogEntryResponse, 0, len(logs))
	for _, log := range logs {
		items = append(items, mapAuditLogToResponse(log))
	}

	return ListIntegrationAuditLogs200JSONResponse(IntegrationAuditLogListResponse{
		Items: items,
		Count: len(items),
	}), nil
}

// ---------------------------------------------------------------------------
// Webhook Ingestion
// ---------------------------------------------------------------------------

func (s *AnchorAPI) IngestWebhook(
	ctx context.Context, request IngestWebhookRequestObject,
) (IngestWebhookResponseObject, error) {
	// Re-marshal the decoded body back to raw bytes for signature validation.
	payload, err := json.Marshal(request.Body)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to marshal webhook body")
		return nil, err
	}

	// Extract HTTP headers from context (injected by WebhookHeadersMiddleware).
	headers := WebhookHeadersFromContext(ctx)

	// The webhook endpoint is unauthenticated — no tenantID in context.
	// The service resolves the tenant via product_id + provider_type lookup.
	input := integration.IngestWebhookInput{
		ProductID:    request.ProductId,
		ProviderType: request.ProviderType,
		Payload:      payload,
		Headers:      headers,
	}

	event, err := s.IntegrationService.IngestWebhook(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("provider_type", string(request.ProviderType)).
			Msg("failed to ingest webhook")
		return nil, err
	}

	return IngestWebhook200JSONResponse(IntegrationWebhookResponse{
		EventId: event.ID,
		Status:  event.Status,
	}), nil
}

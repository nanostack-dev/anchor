package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"

	"anchor/internal/domain/integration"
	"anchor/internal/integration/provider/smtp"
	"anchor/internal/repository"
	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
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
		pub.FromName = functional.OptionOf(cfg.FromName, cfg.FromName != "").ToPtr()
		pub.ReplyTo = functional.OptionOf(cfg.ReplyTo, cfg.ReplyTo != "").ToPtr()
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
	return functional.FromPtr(secret).
		Filter(func(s string) bool { return s != "" }).
		Map(secrets.Obfuscate).
		ToPtr()
}

// legacyAuditLogListLimit is the page the deprecated GET list returns: the
// newest hundred entries, as before.
const legacyAuditLogListLimit int32 = 100

func mapAuditLogToResponse(log integration.AuditLog) IntegrationAuditLogEntryResponse {
	var metadata *map[string]any
	if len(log.MetadataJSON) > 0 {
		parsedMetadata := map[string]any{}
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
		log.ErrorTo(ctx, s.logger, err, "failed to create integration instance",
			slog.String("product_id", request.ProductId),
		)
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
		log.ErrorTo(ctx, s.logger, err, "failed to list integration instances",
			slog.String("product_id", request.ProductId),
		)
		return nil, err
	}

	items := functional.Slice(instances).Map(mapInstanceToResponse).ToSlice()
	if items == nil {
		items = []IntegrationInstanceResponse{}
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
		log.ErrorTo(ctx, s.logger, err, "failed to get integration instance",
			slog.String("product_id", request.ProductId),
			slog.String("integration_instance_id", request.IntegrationInstanceId),
		)
		return nil, err
	}

	if instance == nil {
		return GetIntegrationInstance404JSONResponse{NotFoundJSONResponse(
			notFoundBody("INTEGRATION_INSTANCE_NOT_FOUND", "Integration Instance does not exist."),
		)}, nil
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

	input.IsEnabled = functional.FromPtr(request.Body.IsEnabled).ToPtr()

	if request.Body.Config != nil {
		configJSON, marshalErr := json.Marshal(request.Body.Config)
		if marshalErr != nil {
			return nil, marshalErr
		}
		input.ConfigJSON = configJSON
	}

	updated, err := s.IntegrationService.UpdateInstance(ctx, input)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to update integration instance",
			slog.String("product_id", request.ProductId),
			slog.String("integration_instance_id", request.IntegrationInstanceId),
		)
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
		log.ErrorTo(ctx, s.logger, err, "failed to delete integration instance",
			slog.String("product_id", request.ProductId),
			slog.String("integration_instance_id", request.IntegrationInstanceId),
		)
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

	scope := repository.IntegrationAuditLogScope{
		PlatformTenantID: tenantID, ProductID: request.ProductId, IntegrationInstanceID: request.IntegrationInstanceId,
	}
	q := repository.NewIntegrationAuditLogSearch(scope).
		SortBy(repository.IntegrationAuditLogSortCreatedAt, search.SortDescending).
		Limit(legacyAuditLogListLimit)
	logs, err := s.IntegrationService.SearchAuditLogs(ctx, q)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to list integration audit logs",
			slog.String("product_id", request.ProductId),
			slog.String("integration_instance_id", request.IntegrationInstanceId),
		)
		return nil, err
	}

	items := functional.Slice(logs.Items).Map(mapAuditLogToResponse).ToSlice()
	if items == nil {
		items = []IntegrationAuditLogEntryResponse{}
	}

	return ListIntegrationAuditLogs200JSONResponse(IntegrationAuditLogListResponse{
		Items: items,
		Count: len(items),
	}), nil
}

func (s *AnchorAPI) SearchIntegrationAuditLogs(
	ctx context.Context, request SearchIntegrationAuditLogsRequestObject,
) (SearchIntegrationAuditLogsResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		body = &SearchIntegrationAuditLogsJSONRequestBody{}
	}

	if err = validateAuditLogsSearch(tenantID, request.ProductId, request.IntegrationInstanceId, body); err != nil {
		return nil, err
	}
	q := integrationAuditLogSearch(repository.IntegrationAuditLogScope{
		PlatformTenantID: tenantID, ProductID: request.ProductId, IntegrationInstanceID: request.IntegrationInstanceId,
	}, body)
	result, err := s.IntegrationService.SearchAuditLogs(ctx, q)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search integration audit logs",
			slog.String("product_id", request.ProductId),
			slog.String("integration_instance_id", request.IntegrationInstanceId),
		)
		return nil, err
	}

	return SearchIntegrationAuditLogs200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapAuditLogToResponse),
		Total:      result.Total,
		Count:      result.Count,
		NextCursor: nextCursor(result.NextCursor),
	}, nil
}

// integrationAuditLogSearch builds the search from the request body. The scope
// comes from the authenticated principal and the route; the body never sets it.
// created_after is inclusive and created_before exclusive. full_text_search is
// accepted and ignored: an audit entry has no text column that the contract names.
func integrationAuditLogSearch(
	scope repository.IntegrationAuditLogScope, body *SearchIntegrationAuditLogsJSONRequestBody,
) repository.IntegrationAuditLogSearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	q := repository.NewIntegrationAuditLogSearch(scope)
	q = searchexec.Some(q, q.Action().In, f.Actions)
	q = searchexec.Some(q, q.Severity().In, f.Severities)
	q = searchexec.Opt(q, q.CreatedAt().Since, f.CreatedAfter)
	q = searchexec.Opt(q, q.CreatedAt().Before, f.CreatedBefore)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		repository.IntegrationAuditLogSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
}

// ---------------------------------------------------------------------------
// Webhook Ingestion
// ---------------------------------------------------------------------------

func (s *AnchorAPI) IngestWebhook(
	ctx context.Context, request IngestWebhookRequestObject,
) (IngestWebhookResponseObject, error) {
	payload, ok := webhookPayloadFromContext(ctx)
	if !ok {
		return nil, errors.New("webhook payload middleware is missing")
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
		log.ErrorTo(ctx, s.logger, err, "failed to ingest webhook",
			slog.String("product_id", request.ProductId),
			slog.String("provider_type", string(request.ProviderType)),
		)
		return nil, err
	}

	return IngestWebhook200JSONResponse(IntegrationWebhookResponse{
		EventId: event.ID,
		Status:  event.Status,
	}), nil
}

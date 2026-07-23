package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	"anchor/internal/domain/webhook"
	"anchor/internal/service"
)

// ---------------------------------------------------------------------------
// Mapping
// ---------------------------------------------------------------------------

func mapWebhookEndpointToResponse(endpoint webhook.Endpoint) WebhookEndpointResponse {
	var description *string
	if endpoint.Description != "" {
		description = &endpoint.Description
	}
	var disabledReason *string
	if endpoint.DisabledReason != "" {
		disabledReason = &endpoint.DisabledReason
	}
	eventTypes := endpoint.EventTypes
	if eventTypes == nil {
		eventTypes = []string{}
	}

	return WebhookEndpointResponse{
		Id:                      endpoint.ID,
		ProductId:               endpoint.ProductID,
		Url:                     endpoint.URL,
		Description:             description,
		EventTypes:              eventTypes,
		Status:                  endpoint.Status,
		DisabledReason:          disabledReason,
		ConsecutiveFailureCount: int(endpoint.ConsecutiveFailureCount),
		FirstFailureAt:          endpoint.FirstFailureAt,
		LastFailureAt:           endpoint.LastFailureAt,
		LastSuccessAt:           endpoint.LastSuccessAt,
		CreatedAt:               endpoint.CreatedAt,
		UpdatedAt:               endpoint.UpdatedAt,
	}
}

func mapWebhookEndpointWithSecretToResponse(
	result service.EndpointWithSecret,
) WebhookEndpointWithSecretResponse {
	return WebhookEndpointWithSecretResponse{
		Endpoint: mapWebhookEndpointToResponse(result.Endpoint),
		Secret:   result.PlaintextSecret,
	}
}

func mapWebhookDeliveryToResponse(
	delivery webhook.Delivery, eventType string,
) WebhookDeliveryResponse {
	var lastStatusCode *int
	if delivery.LastStatusCode != nil {
		value := int(*delivery.LastStatusCode)
		lastStatusCode = &value
	}

	return WebhookDeliveryResponse{
		Id:                 delivery.ID,
		EventId:            delivery.EventID,
		EndpointId:         delivery.EndpointID,
		EventType:          eventType,
		Status:             delivery.Status,
		AttemptCount:       int(delivery.AttemptCount),
		MaxAttempts:        int(delivery.MaxAttempts),
		TargetUrl:          delivery.TargetURL,
		LastStatusCode:     lastStatusCode,
		LastError:          delivery.LastError,
		CompletedAt:        delivery.CompletedAt,
		ReplayOfDeliveryId: delivery.ReplayOfDeliveryID,
		CreatedAt:          delivery.CreatedAt,
		UpdatedAt:          delivery.UpdatedAt,
	}
}

func mapWebhookAttemptToResponse(attempt webhook.Attempt) WebhookDeliveryAttemptResponse {
	var statusCode *int
	if attempt.StatusCode != nil {
		value := int(*attempt.StatusCode)
		statusCode = &value
	}

	return WebhookDeliveryAttemptResponse{
		Id:              attempt.ID,
		AttemptNumber:   int(attempt.AttemptNumber),
		StatusCode:      statusCode,
		Error:           attempt.Error,
		ResponseSnippet: attempt.ResponseSnippet,
		DurationMs:      int(attempt.DurationMs),
		AttemptedAt:     attempt.AttemptedAt,
	}
}

// ---------------------------------------------------------------------------
// Endpoint handlers
// ---------------------------------------------------------------------------

func (s *AnchorAPI) ListWebhookEndpoints(
	ctx context.Context, request ListWebhookEndpointsRequestObject,
) (ListWebhookEndpointsResponseObject, error) {
	endpoints, err := s.WebhookEndpointService.List(ctx, webhook.ListEndpointsInput{
		ProductID: request.ProductId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).
			Msg("failed to list webhook endpoints")
		return nil, err
	}

	return ListWebhookEndpoints200JSONResponse{
		Items: slicex.Map(endpoints, mapWebhookEndpointToResponse),
	}, nil
}

func (s *AnchorAPI) CreateWebhookEndpoint(
	ctx context.Context, request CreateWebhookEndpointRequestObject,
) (CreateWebhookEndpointResponseObject, error) {
	var description string
	if request.Body.Description != nil {
		description = *request.Body.Description
	}

	created, err := s.WebhookEndpointService.Create(ctx, webhook.CreateEndpointInput{
		ProductID:   request.ProductId,
		URL:         request.Body.Url,
		Description: description,
		EventTypes:  request.Body.EventTypes,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).
			Msg("failed to create webhook endpoint")
		return nil, err
	}

	return CreateWebhookEndpoint201JSONResponse(
		mapWebhookEndpointWithSecretToResponse(created),
	), nil
}

func (s *AnchorAPI) GetWebhookEndpoint(
	ctx context.Context, request GetWebhookEndpointRequestObject,
) (GetWebhookEndpointResponseObject, error) {
	found, err := s.WebhookEndpointService.Get(ctx, webhook.GetEndpointInput{
		ProductID:  request.ProductId,
		EndpointID: request.WebhookEndpointId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("webhook_endpoint_id", request.WebhookEndpointId).
			Msg("failed to get webhook endpoint")
		return nil, err
	}
	if found == nil {
		return GetWebhookEndpoint404Response{}, nil
	}

	return GetWebhookEndpoint200JSONResponse(mapWebhookEndpointToResponse(*found)), nil
}

func (s *AnchorAPI) UpdateWebhookEndpoint(
	ctx context.Context, request UpdateWebhookEndpointRequestObject,
) (UpdateWebhookEndpointResponseObject, error) {
	updated, err := s.WebhookEndpointService.Update(ctx, webhook.UpdateEndpointInput{
		ProductID:   request.ProductId,
		EndpointID:  request.WebhookEndpointId,
		URL:         request.Body.Url,
		Description: request.Body.Description,
		EventTypes:  request.Body.EventTypes,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("webhook_endpoint_id", request.WebhookEndpointId).
			Msg("failed to update webhook endpoint")
		return nil, err
	}

	return UpdateWebhookEndpoint200JSONResponse(mapWebhookEndpointToResponse(updated)), nil
}

func (s *AnchorAPI) DeleteWebhookEndpoint(
	ctx context.Context, request DeleteWebhookEndpointRequestObject,
) (DeleteWebhookEndpointResponseObject, error) {
	err := s.WebhookEndpointService.Delete(ctx, webhook.DeleteEndpointInput{
		ProductID:  request.ProductId,
		EndpointID: request.WebhookEndpointId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("webhook_endpoint_id", request.WebhookEndpointId).
			Msg("failed to delete webhook endpoint")
		return nil, err
	}

	return DeleteWebhookEndpoint204Response{}, nil
}

func (s *AnchorAPI) EnableWebhookEndpoint(
	ctx context.Context, request EnableWebhookEndpointRequestObject,
) (EnableWebhookEndpointResponseObject, error) {
	updated, err := s.WebhookEndpointService.SetEnabled(ctx, webhook.SetEndpointEnabledInput{
		ProductID:  request.ProductId,
		EndpointID: request.WebhookEndpointId,
		Enabled:    true,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("webhook_endpoint_id", request.WebhookEndpointId).
			Msg("failed to enable webhook endpoint")
		return nil, err
	}

	return EnableWebhookEndpoint200JSONResponse(mapWebhookEndpointToResponse(updated)), nil
}

func (s *AnchorAPI) DisableWebhookEndpoint(
	ctx context.Context, request DisableWebhookEndpointRequestObject,
) (DisableWebhookEndpointResponseObject, error) {
	updated, err := s.WebhookEndpointService.SetEnabled(ctx, webhook.SetEndpointEnabledInput{
		ProductID:  request.ProductId,
		EndpointID: request.WebhookEndpointId,
		Enabled:    false,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("webhook_endpoint_id", request.WebhookEndpointId).
			Msg("failed to disable webhook endpoint")
		return nil, err
	}

	return DisableWebhookEndpoint200JSONResponse(mapWebhookEndpointToResponse(updated)), nil
}

func (s *AnchorAPI) RotateWebhookEndpointSecret(
	ctx context.Context, request RotateWebhookEndpointSecretRequestObject,
) (RotateWebhookEndpointSecretResponseObject, error) {
	rotated, err := s.WebhookEndpointService.RotateSecret(ctx, webhook.RotateSecretInput{
		ProductID:  request.ProductId,
		EndpointID: request.WebhookEndpointId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("webhook_endpoint_id", request.WebhookEndpointId).
			Msg("failed to rotate webhook signing secret")
		return nil, err
	}

	return RotateWebhookEndpointSecret200JSONResponse(
		mapWebhookEndpointWithSecretToResponse(rotated),
	), nil
}

func (s *AnchorAPI) PingWebhookEndpoint(
	ctx context.Context, request PingWebhookEndpointRequestObject,
) (PingWebhookEndpointResponseObject, error) {
	event, err := s.WebhookEndpointService.Ping(ctx, webhook.PingEndpointInput{
		ProductID:  request.ProductId,
		EndpointID: request.WebhookEndpointId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("webhook_endpoint_id", request.WebhookEndpointId).
			Msg("failed to ping webhook endpoint")
		return nil, err
	}

	return PingWebhookEndpoint202JSONResponse{
		EventId:   event.ID,
		EventType: event.EventType,
	}, nil
}

// ---------------------------------------------------------------------------
// Delivery log handlers
// ---------------------------------------------------------------------------

func (s *AnchorAPI) ListWebhookDeliveries(
	ctx context.Context, request ListWebhookDeliveriesRequestObject,
) (ListWebhookDeliveriesResponseObject, error) {
	input := webhook.ListDeliveriesInput{
		ProductID:  request.ProductId,
		EndpointID: request.WebhookEndpointId,
		Status:     request.Params.Status,
		EventType:  request.Params.EventType,
	}
	if request.Params.Limit != nil {
		input.Limit = clampToInt32(*request.Params.Limit)
	}
	if request.Params.Offset != nil {
		input.Offset = clampToInt32(*request.Params.Offset)
	}

	deliveries, err := s.WebhookEndpointService.ListDeliveries(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Str("webhook_endpoint_id", request.WebhookEndpointId).
			Msg("failed to list webhook deliveries")
		return nil, err
	}

	return ListWebhookDeliveries200JSONResponse{
		Items: slicex.Map(deliveries, func(item webhook.DeliveryWithEvent) WebhookDeliveryResponse {
			return mapWebhookDeliveryToResponse(item.Delivery, item.Event.EventType)
		}),
	}, nil
}

func (s *AnchorAPI) GetWebhookDelivery(
	ctx context.Context, request GetWebhookDeliveryRequestObject,
) (GetWebhookDeliveryResponseObject, error) {
	detail, err := s.WebhookEndpointService.GetDelivery(ctx, webhook.GetDeliveryInput{
		ProductID:  request.ProductId,
		EndpointID: request.WebhookEndpointId,
		DeliveryID: request.DeliveryId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("delivery_id", request.DeliveryId).
			Msg("failed to get webhook delivery")
		return nil, err
	}
	if detail == nil {
		return GetWebhookDelivery404Response{}, nil
	}

	return GetWebhookDelivery200JSONResponse{
		Delivery: mapWebhookDeliveryToResponse(detail.Delivery, detail.Event.EventType),
		Payload:  detail.Delivery.SignedBody,
		Attempts: slicex.Map(detail.Attempts, mapWebhookAttemptToResponse),
	}, nil
}

func (s *AnchorAPI) RetryWebhookDelivery(
	ctx context.Context, request RetryWebhookDeliveryRequestObject,
) (RetryWebhookDeliveryResponseObject, error) {
	replay, err := s.WebhookEndpointService.RetryDelivery(ctx, webhook.RetryDeliveryInput{
		ProductID:  request.ProductId,
		EndpointID: request.WebhookEndpointId,
		DeliveryID: request.DeliveryId,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("delivery_id", request.DeliveryId).
			Msg("failed to retry webhook delivery")
		return nil, err
	}

	// The replay row already exists. Its event type is a display convenience, so
	// a lookup failure here degrades the response rather than failing a write
	// that already succeeded.
	eventType := ""
	detail, detailErr := s.WebhookEndpointService.GetDelivery(ctx, webhook.GetDeliveryInput{
		ProductID:  request.ProductId,
		EndpointID: request.WebhookEndpointId,
		DeliveryID: replay.ID,
	})
	switch {
	case detailErr != nil:
		s.logger.Warn().Err(detailErr).Str("delivery_id", replay.ID).
			Msg("replay created but its event could not be read back")
	case detail != nil:
		eventType = detail.Event.EventType
	}

	return RetryWebhookDelivery202JSONResponse(
		mapWebhookDeliveryToResponse(replay, eventType),
	), nil
}

// ---------------------------------------------------------------------------
// Event type catalog
// ---------------------------------------------------------------------------

func (s *AnchorAPI) ListWebhookEventTypes(
	_ context.Context, _ ListWebhookEventTypesRequestObject,
) (ListWebhookEventTypesResponseObject, error) {
	return ListWebhookEventTypes200JSONResponse{
		ApiVersion: webhook.APIVersion,
		Items: slicex.Map(
			webhook.EventTypeCatalog(),
			func(descriptor webhook.EventTypeDescriptor) WebhookEventTypeDescriptor {
				return WebhookEventTypeDescriptor{
					Type:        descriptor.Type,
					Group:       descriptor.Group,
					Description: descriptor.Description,
				}
			},
		),
	}, nil
}

// clampToInt32 narrows a query-string integer without wrapping. Values outside
// the range keep the out-of-range boundary so the service validator answers
// them with a 400 rather than the narrowing turning them into valid input.
func clampToInt32(value int) int32 {
	const (
		minInt32 = -1 << 31
		maxInt32 = 1<<31 - 1
	)

	switch {
	case value < minInt32:
		return minInt32
	case value > maxInt32:
		return maxInt32
	default:
		return int32(value)
	}
}

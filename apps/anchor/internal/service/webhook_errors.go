package service

import (
	"fmt"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

func NewInvalidWebhookURLError(reason string) *fault.Error {
	return fault.BadRequest("INVALID_WEBHOOK_URL", reason)
}

func NewInvalidWebhookEventTypesError(reason string) *fault.Error {
	return fault.BadRequest("INVALID_WEBHOOK_EVENT_TYPES", reason)
}

func NewWebhookEndpointNotEnabledError(endpointID string) *fault.Error {
	return fault.BadRequest(
		"WEBHOOK_ENDPOINT_NOT_ENABLED",
		fmt.Sprintf("Webhook endpoint %s is not enabled", endpointID),
	).Metadata(map[string]any{"webhook_endpoint_id": endpointID})
}

// NewWebhookDeliveryNotReplayableError rejects a replay of a replay. Keeping
// the log one hop deep means a delivery always points at the original attempt
// chain rather than an arbitrarily long chain of retries of retries.
func NewWebhookDeliveryNotReplayableError(deliveryID string) *fault.Error {
	return fault.BadRequest(
		"WEBHOOK_DELIVERY_NOT_REPLAYABLE",
		fmt.Sprintf(
			"Delivery %s is itself a replay; retry the original delivery instead",
			deliveryID,
		),
	).Metadata(map[string]any{"delivery_id": deliveryID})
}

func NewWebhookDeliveryStillPendingError(deliveryID string) *fault.Error {
	return fault.NewWithStatus(
		"WEBHOOK_DELIVERY_STILL_PENDING",
		fmt.Sprintf("Delivery %s has not finished its own attempts yet", deliveryID),
		http.StatusConflict,
	).Metadata(map[string]any{"delivery_id": deliveryID})
}

func NewWebhookSecretGenerationError(err error) *fault.Error {
	return fault.NewWithStatus(
		"WEBHOOK_SECRET_GENERATION_FAILED",
		"Failed to generate a webhook signing secret: "+err.Error(),
		http.StatusInternalServerError,
	)
}

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/rs/zerolog"

	"anchor/internal/domain/webhook"
	"anchor/internal/repository"
	"anchor/internal/security/encryption"
)

// WebhookSigningSecretContext isolates webhook signing secrets from every other
// secret class encrypted with the app key.
const WebhookSigningSecretContext = "webhook-signing-secret"

// WebhookDeliveryService performs one signed POST per `webhook.deliver` job and
// records what happened.
type WebhookDeliveryService interface {
	// ProcessQueueJob handles a `webhook.deliver` job.
	ProcessQueueJob(ctx context.Context, job pgqueue.Job) error
	// DeliverOnce performs a single attempt for a delivery. It returns the
	// queue-facing error: nil on success, a plain error to retry, and a
	// pgqueue.NonRetryable error when the delivery is finished for good.
	DeliverOnce(ctx context.Context, deliveryID string) error
}

type webhookDeliveryService struct {
	deliveryRepo repository.WebhookDeliveryRepository
	endpointRepo repository.WebhookEndpointRepository
	secretRepo   repository.WebhookEndpointSecretRepository
	cipher       *secrets.VersionedCipher
	httpClient   *WebhookHTTPClient
	logger       zerolog.Logger
}

func NewWebhookDeliveryService(
	deliveryRepo repository.WebhookDeliveryRepository,
	endpointRepo repository.WebhookEndpointRepository,
	secretRepo repository.WebhookEndpointSecretRepository,
	encryptionService *encryption.Service,
	httpClient *WebhookHTTPClient,
	logger zerolog.Logger,
) (WebhookDeliveryService, error) {
	cipher, err := encryptionService.NewCipher(WebhookSigningSecretContext)
	if err != nil {
		return nil, fmt.Errorf("build webhook signing secret cipher: %w", err)
	}

	return &webhookDeliveryService{
		deliveryRepo: deliveryRepo,
		endpointRepo: endpointRepo,
		secretRepo:   secretRepo,
		cipher:       cipher,
		httpClient:   httpClient,
		logger:       logger.With().Str("component", "webhook_delivery_service").Logger(),
	}, nil
}

func (s *webhookDeliveryService) ProcessQueueJob(ctx context.Context, job pgqueue.Job) error {
	var payload webhookDeliverPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return pgqueue.NonRetryable(fmt.Errorf("invalid webhook deliver payload: %w", err))
	}
	if payload.DeliveryID == "" {
		return pgqueue.NonRetryable(errors.New("webhook deliver payload missing delivery_id"))
	}

	return s.DeliverOnce(ctx, payload.DeliveryID)
}

func (s *webhookDeliveryService) DeliverOnce(ctx context.Context, deliveryID string) error {
	logger := s.logger.With().Str("delivery_id", deliveryID).Logger()

	delivery, err := s.deliveryRepo.FindByIDInternal(ctx, deliveryID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to load webhook delivery")
		return err
	}
	if delivery == nil {
		return pgqueue.NonRetryable(errors.New("webhook delivery not found"))
	}
	if delivery.Status.IsTerminal() {
		// A reaped or duplicated job for an already-finished delivery. At-least
		// -once is the only practical guarantee, so this is expected traffic,
		// not an error.
		logger.Debug().Str("status", string(delivery.Status)).
			Msg("webhook delivery already finished; skipping")
		return nil
	}

	endpoint, err := s.endpointRepo.FindByIDInternal(ctx, delivery.EndpointID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to load webhook endpoint")
		return err
	}
	if endpoint == nil {
		return s.finish(ctx, *delivery, webhook.DeliveryStatusFailed, nil,
			new("webhook endpoint no longer exists"), pgqueue.NonRetryable(
				errors.New("webhook endpoint no longer exists"),
			))
	}
	if !endpoint.IsEnabled() {
		reason := "webhook endpoint is " + string(endpoint.Status)
		return s.finish(ctx, *delivery, webhook.DeliveryStatusFailed, nil,
			&reason, pgqueue.NonRetryable(errors.New(reason)))
	}

	headers, err := s.signatureHeaders(ctx, *delivery)
	if err != nil {
		logger.Error().Err(err).Msg("failed to build webhook signature headers")
		return err
	}

	response, transportErr := s.httpClient.Post(
		ctx, delivery.TargetURL, delivery.SignedBody, headers,
	)

	attemptNumber := delivery.AttemptCount + 1
	s.recordAttempt(ctx, logger, *delivery, attemptNumber, response, transportErr)

	outcome := webhook.Classify(response.StatusCode, transportErr)
	delivery.AttemptCount = attemptNumber

	return s.applyOutcome(ctx, logger, *delivery, endpoint, outcome, response, transportErr)
}

// signatureHeaders builds the Standard Webhooks headers for one attempt. The
// timestamp is fresh per attempt so a captured request falls outside a
// receiver's tolerance window; the delivery id is stable across retries so a
// receiver can use it as an idempotency key.
func (s *webhookDeliveryService) signatureHeaders(
	ctx context.Context, delivery webhook.Delivery,
) (map[string]string, error) {
	stored, err := s.secretRepo.ListByEndpointInternal(ctx, delivery.EndpointID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	usable := webhook.UsableSecrets(stored, now)
	plaintexts := make([]string, 0, len(usable))
	for _, secret := range usable {
		plaintext, decryptErr := s.cipher.DecryptString(secret.EncryptedSecret)
		if decryptErr != nil {
			s.logger.Error().Err(decryptErr).
				Str("webhook_secret_id", secret.ID).
				Msg("failed to decrypt webhook signing secret")
			continue
		}
		plaintexts = append(plaintexts, plaintext)
	}
	if len(plaintexts) == 0 {
		return nil, errors.New("webhook endpoint has no usable signing secret")
	}

	timestamp := now.Unix()

	return map[string]string{
		webhook.HeaderWebhookID:        delivery.ID,
		webhook.HeaderWebhookTimestamp: strconv.FormatInt(timestamp, 10),
		webhook.HeaderWebhookSignature: webhook.SignatureHeader(
			plaintexts, delivery.ID, timestamp, delivery.SignedBody,
		),
	}, nil
}

// recordAttempt appends the immutable attempt row. A failure to write the log
// must not abort the delivery itself, so it is logged rather than returned.
func (s *webhookDeliveryService) recordAttempt(
	ctx context.Context,
	logger zerolog.Logger,
	delivery webhook.Delivery,
	attemptNumber int32,
	response WebhookHTTPResponse,
	transportErr error,
) {
	attempt := webhook.Attempt{
		DeliveryID:    delivery.ID,
		AttemptNumber: attemptNumber,
		DurationMs:    clampDurationMs(response.Duration),
		AttemptedAt:   time.Now(),
		CreatedAt:     time.Now(),
	}
	attempt.GenerateID()

	if transportErr != nil {
		message := webhook.TruncateError(transportErr.Error())
		attempt.Error = &message
	} else {
		statusCode := clampStatusCode(response.StatusCode)
		attempt.StatusCode = &statusCode
		if response.Snippet != "" {
			snippet := response.Snippet
			attempt.ResponseSnippet = &snippet
		}
	}

	if _, err := s.deliveryRepo.CreateAttemptInternal(ctx, attempt); err != nil {
		logger.Error().Err(err).Msg("failed to record webhook delivery attempt")
	}
}

// applyOutcome writes the delivery's new state, updates the endpoint's health
// counters, and returns the error pgqueue needs to decide on a retry.
func (s *webhookDeliveryService) applyOutcome(
	ctx context.Context,
	logger zerolog.Logger,
	delivery webhook.Delivery,
	endpoint *webhook.Endpoint,
	outcome webhook.Outcome,
	response WebhookHTTPResponse,
	transportErr error,
) error {
	statusCode, failureMessage := attemptSummary(response, transportErr)

	switch outcome {
	case webhook.OutcomeSucceeded:
		s.updateEndpointHealth(ctx, logger, endpoint, true)
		return s.finish(ctx, delivery, webhook.DeliveryStatusSucceeded, statusCode, nil, nil)

	case webhook.OutcomeDisableEndpoint:
		s.disableEndpoint(ctx, logger, endpoint, webhook.GoneDisableReason)
		return s.finish(ctx, delivery, webhook.DeliveryStatusFailed, statusCode,
			failureMessage, pgqueue.NonRetryable(
				errors.New("receiver answered 410 gone; endpoint disabled"),
			))

	case webhook.OutcomeFailed:
		s.updateEndpointHealth(ctx, logger, endpoint, false)
		return s.finish(ctx, delivery, webhook.DeliveryStatusFailed, statusCode,
			failureMessage, pgqueue.NonRetryable(
				errors.New(derefOr(failureMessage, "permanent delivery failure")),
			))

	case webhook.OutcomeRetry:
		s.updateEndpointHealth(ctx, logger, endpoint, false)
		if delivery.AttemptsRemaining() {
			return s.parkForRetry(ctx, delivery, statusCode, failureMessage)
		}

		logger.Warn().Int32("attempts", delivery.AttemptCount).
			Msg("webhook delivery exhausted its retry ladder")

		return s.finish(ctx, delivery, webhook.DeliveryStatusExhausted, statusCode,
			failureMessage, pgqueue.NonRetryable(
				errors.New(derefOr(failureMessage, "retries exhausted")),
			))

	default:
		return pgqueue.NonRetryable(
			fmt.Errorf("unknown webhook delivery outcome %q", outcome),
		)
	}
}

// parkForRetry keeps the delivery PENDING and hands a retryable error back to
// pgqueue, which reschedules it on the jittered ladder.
func (s *webhookDeliveryService) parkForRetry(
	ctx context.Context,
	delivery webhook.Delivery,
	statusCode *int32,
	failureMessage *string,
) error {
	if err := s.deliveryRepo.UpdateOutcomeInternal(ctx, delivery.ID, repository.DeliveryOutcome{
		Status:         webhook.DeliveryStatusPending,
		AttemptCount:   delivery.AttemptCount,
		LastStatusCode: statusCode,
		LastError:      failureMessage,
	}); err != nil {
		return err
	}

	return errors.New(derefOr(failureMessage, "webhook delivery attempt failed"))
}

// finish writes a terminal (or, for PENDING, an interim) delivery state and
// returns the queue-facing error unchanged.
func (s *webhookDeliveryService) finish(
	ctx context.Context,
	delivery webhook.Delivery,
	status webhook.DeliveryStatus,
	statusCode *int32,
	failureMessage *string,
	queueErr error,
) error {
	completedAt := time.Now()
	outcome := repository.DeliveryOutcome{
		Status:         status,
		AttemptCount:   delivery.AttemptCount,
		LastStatusCode: statusCode,
		LastError:      failureMessage,
		CompletedAt:    &completedAt,
	}

	if err := s.deliveryRepo.UpdateOutcomeInternal(ctx, delivery.ID, outcome); err != nil {
		s.logger.Error().Err(err).Str("delivery_id", delivery.ID).
			Msg("failed to write webhook delivery outcome")
		return err
	}

	return queueErr
}

// updateEndpointHealth advances or clears the endpoint's failure streak and
// applies the two-condition auto-disable rule.
func (s *webhookDeliveryService) updateEndpointHealth(
	ctx context.Context, logger zerolog.Logger, endpoint *webhook.Endpoint, succeeded bool,
) {
	now := time.Now()
	counters := webhook.CountersOf(endpoint)

	status := endpoint.Status
	reason := endpoint.DisabledReason

	if succeeded {
		counters = webhook.RecordSuccess(counters, now)
		reason = ""
	} else {
		counters = webhook.RecordFailure(counters, now)
		if webhook.ShouldAutoDisable(counters, now) {
			status = webhook.EndpointStatusAutoDisabled
			reason = webhook.AutoDisableReason
			logger.Warn().Str("webhook_endpoint_id", endpoint.ID).
				Int32("consecutive_failures", counters.ConsecutiveFailureCount).
				Msg("webhook endpoint auto-disabled after sustained failures")
		}
	}

	if err := s.endpointRepo.UpdateHealthInternal(
		ctx, endpoint.ID, counters, status, reason,
	); err != nil {
		logger.Error().Err(err).Str("webhook_endpoint_id", endpoint.ID).
			Msg("failed to update webhook endpoint health")
	}
}

// disableEndpoint turns an endpoint off immediately, used when the receiver
// itself declared it gone.
func (s *webhookDeliveryService) disableEndpoint(
	ctx context.Context, logger zerolog.Logger, endpoint *webhook.Endpoint, reason string,
) {
	now := time.Now()
	counters := webhook.RecordFailure(webhook.CountersOf(endpoint), now)

	if err := s.endpointRepo.UpdateHealthInternal(
		ctx, endpoint.ID, counters, webhook.EndpointStatusAutoDisabled, reason,
	); err != nil {
		logger.Error().Err(err).Str("webhook_endpoint_id", endpoint.ID).
			Msg("failed to disable webhook endpoint")
		return
	}

	logger.Warn().Str("webhook_endpoint_id", endpoint.ID).Str("reason", reason).
		Msg("webhook endpoint disabled")
}

// attemptSummary reduces an attempt result to what the delivery row stores.
func attemptSummary(response WebhookHTTPResponse, transportErr error) (*int32, *string) {
	if transportErr != nil {
		message := webhook.TruncateError(transportErr.Error())
		return nil, &message
	}

	statusCode := clampStatusCode(response.StatusCode)
	message := webhook.TruncateError(
		"receiver responded with HTTP " + strconv.Itoa(response.StatusCode),
	)

	return &statusCode, &message
}

// clampStatusCode narrows an HTTP status code into the persisted column width.
// Real status codes are three digits; the clamp is what lets the conversion be
// provably safe rather than merely assumed to be.
func clampStatusCode(statusCode int) int32 {
	if statusCode < 0 {
		return 0
	}
	if statusCode > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(statusCode)
}

// clampDurationMs narrows an attempt duration into the persisted column width.
// The request timeout keeps real values far below the ceiling; the clamp exists
// so a pathological clock cannot wrap the stored value negative.
func clampDurationMs(duration time.Duration) int32 {
	milliseconds := duration.Milliseconds()
	if milliseconds < 0 {
		return 0
	}
	if milliseconds > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(milliseconds)
}

func derefOr(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}

	return *value
}

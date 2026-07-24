package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/rs/zerolog"

	"anchor/internal/domain/webhook"
	"anchor/internal/repository"
)

// WebhookFanoutService turns one emitted event into the set of deliveries it
// produces: one per subscribed, enabled endpoint of the product.
type WebhookFanoutService interface {
	// ProcessQueueJob handles a `webhook.fanout` job.
	ProcessQueueJob(ctx context.Context, job pgqueue.Job) error
	// FanOutEvent is the same work addressed directly by event id. Tests and
	// the queue handler share this entry point.
	FanOutEvent(ctx context.Context, eventID string) ([]webhook.Delivery, error)
	// CreateDeliveriesInTx fans an event out on the CALLER'S transaction.
	//
	// It exists for the test-event sub-resource, which has to answer with the
	// delivery id it created. Doing that work in the emitting transaction is
	// what makes the answer both immediate and race-free: the queued fan-out
	// job only becomes visible at the same commit, and finds the rows already
	// there.
	CreateDeliveriesInTx(
		ctx context.Context, event webhook.Event, endpoints []webhook.Endpoint,
	) ([]webhook.Delivery, error)
}

type webhookFanoutService struct {
	eventRepo    repository.WebhookEventRepository
	endpointRepo repository.WebhookEndpointRepository
	deliveryRepo repository.WebhookDeliveryRepository
	queue        *pgqueue.Client
	transactor   transactor.Transactor
	logger       zerolog.Logger
}

func NewWebhookFanoutService(
	eventRepo repository.WebhookEventRepository,
	endpointRepo repository.WebhookEndpointRepository,
	deliveryRepo repository.WebhookDeliveryRepository,
	queue *pgqueue.Client,
	txr transactor.Transactor,
	logger zerolog.Logger,
) WebhookFanoutService {
	return &webhookFanoutService{
		eventRepo:    eventRepo,
		endpointRepo: endpointRepo,
		deliveryRepo: deliveryRepo,
		queue:        queue,
		transactor:   txr,
		logger:       logger.With().Str("component", "webhook_fanout_service").Logger(),
	}
}

func (s *webhookFanoutService) ProcessQueueJob(ctx context.Context, job pgqueue.Job) error {
	var payload webhookFanoutPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return pgqueue.NonRetryable(fmt.Errorf("invalid webhook fanout payload: %w", err))
	}
	if payload.EventID == "" {
		return pgqueue.NonRetryable(errors.New("webhook fanout payload missing event_id"))
	}

	_, err := s.FanOutEvent(ctx, payload.EventID)

	return err
}

func (s *webhookFanoutService) FanOutEvent(
	ctx context.Context, eventID string,
) ([]webhook.Delivery, error) {
	logger := s.logger.With().Str("event_id", eventID).Logger()

	event, err := s.eventRepo.FindByIDInternal(ctx, eventID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to load webhook event")
		return nil, err
	}
	if event == nil {
		// The event row is gone — its product or organization was deleted while
		// the job waited. There is nothing to deliver and nothing to retry.
		logger.Warn().Msg("webhook event no longer exists; dropping fanout job")
		return nil, pgqueue.NonRetryable(errors.New("webhook event not found"))
	}

	endpoints, err := s.matchingEndpoints(ctx, *event)
	if err != nil {
		return nil, err
	}
	if len(endpoints) == 0 {
		logger.Debug().Str("event_type", event.EventType).
			Msg("no matching webhook endpoints; nothing to deliver")
		return nil, nil
	}

	var created []webhook.Delivery
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		var txErr error
		created, txErr = s.CreateDeliveriesInTx(txCtx, *event, endpoints)
		return txErr
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to fan out webhook event")
		return nil, err
	}

	logger.Info().
		Str("event_type", event.EventType).
		Int("deliveries", len(created)).
		Msg("webhook event fanned out")

	return created, nil
}

// matchingEndpoints resolves the endpoints an event should reach. A synthetic
// event carrying a target endpoint (ping) reaches exactly that one endpoint;
// every other event reaches the product's enabled subscribers.
func (s *webhookFanoutService) matchingEndpoints(
	ctx context.Context, event webhook.Event,
) ([]webhook.Endpoint, error) {
	if event.TargetEndpointID != nil {
		endpoint, err := s.endpointRepo.FindByIDInternal(ctx, *event.TargetEndpointID)
		if err != nil {
			return nil, err
		}
		if endpoint == nil || !endpoint.IsEnabled() {
			return nil, nil
		}

		return []webhook.Endpoint{*endpoint}, nil
	}

	endpoints, err := s.endpointRepo.ListEnabledByProductInternal(ctx, event.ProductID)
	if err != nil {
		s.logger.Error().Err(err).Str("product_id", event.ProductID).
			Msg("failed to list webhook endpoints")
		return nil, err
	}

	matching := make([]webhook.Endpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.Subscribes(event.EventType) {
			matching = append(matching, endpoint)
		}
	}

	return matching, nil
}

func (s *webhookFanoutService) CreateDeliveriesInTx(
	ctx context.Context, event webhook.Event, endpoints []webhook.Endpoint,
) ([]webhook.Delivery, error) {
	body, err := json.Marshal(event.Envelope())
	if err != nil {
		return nil, pgqueue.NonRetryable(fmt.Errorf("marshal webhook envelope: %w", err))
	}

	return s.createDeliveries(ctx, event, endpoints, string(body))
}

// createDeliveries inserts one delivery per endpoint and enqueues its job.
//
// The unique index on (event_id, endpoint_id) for non-replay rows is what makes
// this idempotent: a fan-out re-run after a crash finds the existing row and
// skips it instead of double-delivering.
func (s *webhookFanoutService) createDeliveries(
	ctx context.Context,
	event webhook.Event,
	endpoints []webhook.Endpoint,
	body string,
) ([]webhook.Delivery, error) {
	tx := transactor.CurrentTx(ctx)
	if tx == nil {
		return nil, errors.New("webhook fanout requires an ambient transaction")
	}

	created := make([]webhook.Delivery, 0, len(endpoints))
	for _, endpoint := range endpoints {
		existing, err := s.deliveryRepo.FindOriginalByEventAndEndpointInternal(
			ctx, event.ID, endpoint.ID,
		)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			continue
		}

		now := time.Now()
		delivery := webhook.Delivery{
			EventID:      event.ID,
			EndpointID:   endpoint.ID,
			ProductID:    event.ProductID,
			Status:       webhook.DeliveryStatusPending,
			AttemptCount: 0,
			MaxAttempts:  webhook.MaxDeliveryAttempts,
			TargetURL:    endpoint.URL,
			SignedBody:   body,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		delivery.GenerateID()

		stored, err := s.deliveryRepo.CreateInternal(ctx, delivery)
		if err != nil {
			return nil, err
		}

		if err = enqueueWebhookDelivery(ctx, s.queue, tx, stored.ID); err != nil {
			return nil, err
		}

		created = append(created, stored)
	}

	return created, nil
}

// enqueueWebhookDelivery adds a delivery job on the supplied transaction, so
// the job and the delivery row commit together.
func enqueueWebhookDelivery(
	ctx context.Context, queue *pgqueue.Client, tx *sql.Tx, deliveryID string,
) error {
	payload, err := json.Marshal(webhookDeliverPayload{DeliveryID: deliveryID})
	if err != nil {
		return fmt.Errorf("marshal webhook deliver payload: %w", err)
	}

	_, err = queue.EnqueueTx(ctx, tx, pgqueue.EnqueueParams{
		QueueName:   WebhookDeliverQueueName,
		Payload:     payload,
		MaxAttempts: int(webhook.MaxDeliveryAttempts),
	})

	return err
}

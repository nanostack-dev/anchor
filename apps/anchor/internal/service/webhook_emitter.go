package service

import (
	"context"
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

const (
	// WebhookFanoutQueueName carries one job per emitted event. Fan-out is fast
	// and touches only the database.
	WebhookFanoutQueueName = "webhook.fanout"

	// WebhookDeliverQueueName carries one job per delivery. Delivery is slow and
	// touches the network; keeping it on its own queue stops a single
	// unreachable endpoint from parking every other product's events behind it.
	WebhookDeliverQueueName = "webhook.deliver"

	webhookFanoutMaxAttempts = 5
)

// webhookFanoutPayload is the fan-out job body.
type webhookFanoutPayload struct {
	EventID string `json:"event_id"`
}

// webhookDeliverPayload is the delivery job body.
type webhookDeliverPayload struct {
	DeliveryID string `json:"delivery_id"`
}

// WebhookEmitter is the entire seam other features use to publish an event.
//
// It writes the outbox row and enqueues the fan-out job inside the CALLER'S
// transaction, so an event exists if and only if the business change that
// produced it committed. No HTTP call ever happens inside that transaction: an
// HTTP call cannot be rolled back, and holding a connection across a
// fifteen-second POST exhausts the pool.
//
// The interface stays narrow on purpose. Adding an event type means adding a
// registry constant and one Emit call at the business site — nothing here, and
// nothing in fan-out, signing, retry or delivery, changes.
type WebhookEmitter interface {
	Emit(ctx context.Context, input webhook.EmitInput) (webhook.Event, error)
}

type webhookEmitter struct {
	eventRepo  repository.WebhookEventRepository
	queue      *pgqueue.Client
	transactor transactor.Transactor
	logger     zerolog.Logger
}

func NewWebhookEmitter(
	eventRepo repository.WebhookEventRepository,
	queue *pgqueue.Client,
	txr transactor.Transactor,
	logger zerolog.Logger,
) WebhookEmitter {
	return &webhookEmitter{
		eventRepo:  eventRepo,
		queue:      queue,
		transactor: txr,
		logger:     logger.With().Str("component", "webhook_emitter").Logger(),
	}
}

func (s *webhookEmitter) Emit(
	ctx context.Context, input webhook.EmitInput,
) (webhook.Event, error) {
	if err := validateStruct(input); err != nil {
		return webhook.Event{}, err
	}
	if err := webhook.Validate(input.EventType); err != nil {
		return webhook.Event{}, NewInvalidWebhookEventTypesError(err.Error())
	}

	payload, err := json.Marshal(input.Data)
	if err != nil {
		return webhook.Event{}, fmt.Errorf("marshal webhook event data: %w", err)
	}
	if input.Data == nil {
		payload = json.RawMessage(`{}`)
	}

	now := time.Now().UTC()
	event := webhook.Event{
		ProductID:        input.ProductID,
		OrganizationID:   input.OrganizationID,
		EventType:        input.EventType,
		APIVersion:       webhook.APIVersion,
		Payload:          payload,
		OccurredAt:       now,
		TargetEndpointID: input.TargetEndpointID,
		CreatedAt:        now,
	}
	event.GenerateID()

	// Emit is normally called from inside another service's InTx closure. When
	// it is not (a standalone emit), open a transaction so the outbox row and
	// its job still commit together.
	if transactor.CurrentTx(ctx) != nil {
		return event, s.persist(ctx, event)
	}

	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		return s.persist(txCtx, event)
	})

	return event, err
}

// persist writes the outbox row and its fan-out job on the ambient transaction.
func (s *webhookEmitter) persist(ctx context.Context, event webhook.Event) error {
	tx := transactor.CurrentTx(ctx)
	if tx == nil {
		return errors.New("webhook emit requires an ambient transaction")
	}

	created, err := s.eventRepo.CreateInternal(ctx, event)
	if err != nil {
		s.logger.Error().Err(err).
			Str("event_type", event.EventType).
			Msg("failed to write webhook outbox row")
		return err
	}

	jobPayload, err := json.Marshal(webhookFanoutPayload{EventID: created.ID})
	if err != nil {
		return fmt.Errorf("marshal webhook fanout payload: %w", err)
	}

	if _, err = s.queue.EnqueueTx(ctx, tx, pgqueue.EnqueueParams{
		QueueName:   WebhookFanoutQueueName,
		Payload:     jobPayload,
		MaxAttempts: webhookFanoutMaxAttempts,
	}); err != nil {
		s.logger.Error().Err(err).
			Str("event_id", created.ID).
			Msg("failed to enqueue webhook fanout job")
		return err
	}

	s.logger.Debug().
		Str("event_id", created.ID).
		Str("event_type", created.EventType).
		Str("product_id", created.ProductID).
		Msg("webhook event emitted")

	return nil
}

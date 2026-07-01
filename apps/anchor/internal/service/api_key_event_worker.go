package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const (
	organizationAPIKeyEventQueueName      = "organization_api_key_event"
	organizationAPIKeyEventWorkerID       = "anchor-organization-api-key-worker"
	organizationAPIKeyEventPollInterval   = 2 * time.Second
	organizationAPIKeyEventReapInterval   = 30 * time.Second
	organizationAPIKeyEventVisibility     = 1 * time.Minute
	organizationAPIKeyEventBatchSize      = 50
	organizationAPIKeyEventBackoffBase    = 1 * time.Second
	organizationAPIKeyEventBackoffMax     = 5 * time.Minute
	organizationAPIKeyEventMaxAttempts    = 6
	organizationAPIKeyEventListLimit      = 1000
	organizationAPIKeyEventTypeExpiration = "expiration"
)

type organizationAPIKeyEventPayload struct {
	EventType      string `json:"event_type"`
	OrganizationID string `json:"organization_id"`
	APIKeyID       string `json:"api_key_id"`
}

type APIKeyEventWorkerParams struct {
	fx.In
	Lifecycle                      fx.Lifecycle
	OrganizationAPIKeyEventService OrganizationAPIKeyEventService
	Queue                          *pgqueue.Client
	Logger                         zerolog.Logger
}

func RegisterAPIKeyEventWorker(p APIKeyEventWorkerParams) {
	logger := p.Logger.With().Str("component", "organization_api_key_event_worker").Logger()

	registry := pgqueue.NewHandlerRegistry()
	if err := registry.Register(
		organizationAPIKeyEventQueueName, func(ctx context.Context, job pgqueue.Job) error {
			return p.OrganizationAPIKeyEventService.ProcessQueueJob(ctx, job)
		},
	); err != nil {
		logger.Error().Err(err).Msg("failed to register organization api key event queue handler")
		return
	}

	worker, err := pgqueue.NewWorker(
		p.Queue, registry, pgqueue.WorkerConfig{
			WorkerID:          organizationAPIKeyEventWorkerID,
			PollInterval:      organizationAPIKeyEventPollInterval,
			ReapInterval:      organizationAPIKeyEventReapInterval,
			VisibilityTimeout: organizationAPIKeyEventVisibility,
			BatchSizePerQueue: organizationAPIKeyEventBatchSize,
			BackoffBase:       organizationAPIKeyEventBackoffBase,
			BackoffMax:        organizationAPIKeyEventBackoffMax,
			OnJobFailed: func(_ context.Context, job pgqueue.Job) {
				logger.Error().
					Int64("job_id", job.ID).
					Str("queue", job.QueueName).
					Int("attempts", job.Attempts).
					Str("last_error", job.LastError.String).
					Msg("organization api key event job permanently failed")
			},
			OnJobStuck: func(_ context.Context, result pgqueue.ReapResult) {
				logger.WithLevel(reapLogLevel(result)).
					Int64("requeued", result.Requeued).
					Int64("failed", result.Failed).
					Msg("organization api key event jobs stuck in processing were reaped")
			},
		},
	)
	if err != nil {
		logger.Error().Err(err).Msg("failed to initialize organization api key event worker")
		return
	}

	var cancel context.CancelFunc

	p.Lifecycle.Append(
		fx.Hook{
			OnStart: func(_ context.Context) error {
				workerCtx, workerCancel := newOrganizationAPIKeyWorkerContext()
				cancel = workerCancel

				go func() {
					if runErr := worker.Run(workerCtx); runErr != nil {
						logger.Error().Err(runErr).Msg("organization api key event worker stopped with error")
					}
				}()

				logger.Info().
					Str("queue_name", organizationAPIKeyEventQueueName).
					Dur("poll_interval", organizationAPIKeyEventPollInterval).
					Int("batch_size", organizationAPIKeyEventBatchSize).
					Int("max_attempts", organizationAPIKeyEventMaxAttempts).
					Msg("organization api key event worker started")
				return nil
			},
			OnStop: func(_ context.Context) error {
				logger.Info().Msg("stopping organization api key event worker")
				if cancel != nil {
					cancel()
				}
				return nil
			},
		},
	)
}

func (s *organizationAPIKeyEventService) ProcessQueueJob(
	ctx context.Context, job pgqueue.Job,
) error {
	var payload organizationAPIKeyEventPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return pgqueue.NonRetryable(
			fmt.Errorf(
				"invalid organization api key event payload: %w", err,
			),
		)
	}
	if payload.EventType == "" {
		return pgqueue.NonRetryable(errors.New("organization api key event payload missing event_type"))
	}
	if payload.OrganizationID == "" {
		return pgqueue.NonRetryable(errors.New("organization api key event payload missing organization_id"))
	}
	if payload.APIKeyID == "" {
		return pgqueue.NonRetryable(errors.New("organization api key event payload missing api_key_id"))
	}

	switch payload.EventType {
	case organizationAPIKeyEventTypeExpiration:
		return s.processOrganizationAPIKeyExpiration(ctx, payload)
	default:
		return pgqueue.NonRetryable(
			fmt.Errorf(
				"unsupported organization api key event type: %s", payload.EventType,
			),
		)
	}
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

func newOrganizationAPIKeyWorkerContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/pgkit/queue"
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
	Queue                          *queue.Client
	Logger                         *slog.Logger
}

func RegisterAPIKeyEventWorker(p APIKeyEventWorkerParams) {
	logger := p.Logger.With(slog.String("component", "organization_api_key_event_worker"))

	registry := queue.NewHandlerRegistry()
	if err := registry.Register(
		organizationAPIKeyEventQueueName, func(ctx context.Context, job queue.Job) error {
			return p.OrganizationAPIKeyEventService.ProcessQueueJob(ctx, job)
		},
	); err != nil {
		logger.Error("failed to register organization api key event queue handler", log.Err(err))
		return
	}

	worker, err := queue.NewWorker(
		p.Queue, registry, queue.WorkerConfig{
			WorkerID:          organizationAPIKeyEventWorkerID,
			PollInterval:      organizationAPIKeyEventPollInterval,
			ReapInterval:      organizationAPIKeyEventReapInterval,
			VisibilityTimeout: organizationAPIKeyEventVisibility,
			BatchSizePerQueue: organizationAPIKeyEventBatchSize,
			BackoffBase:       organizationAPIKeyEventBackoffBase,
			BackoffMax:        organizationAPIKeyEventBackoffMax,
			OnJobFailed: func(ctx context.Context, job queue.Job) {
				logger.ErrorContext(ctx, "organization api key event job permanently failed",
					slog.Int64("job_id", job.ID),
					slog.String("queue", job.QueueName),
					slog.Int("attempts", job.Attempts),
					slog.String("last_error", job.LastError.String),
				)
			},
			OnJobStuck: func(ctx context.Context, result queue.ReapResult) {
				logger.LogAttrs(ctx, reapLogLevel(result),
					"organization api key event jobs stuck in processing were reaped",
					slog.Int64("requeued", result.Requeued),
					slog.Int64("failed", result.Failed),
				)
			},
		},
	)
	if err != nil {
		logger.Error("failed to initialize organization api key event worker", log.Err(err))
		return
	}

	var cancel context.CancelFunc

	p.Lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				workerCtx, workerCancel := newOrganizationAPIKeyWorkerContext()
				cancel = workerCancel

				go func() {
					if runErr := worker.Run(workerCtx); runErr != nil {
						logger.ErrorContext(
							workerCtx,
							"organization api key event worker stopped with error",
							log.Err(runErr),
						)
					}
				}()

				logger.InfoContext(ctx, "organization api key event worker started",
					slog.String("queue_name", organizationAPIKeyEventQueueName),
					slog.Duration("poll_interval", organizationAPIKeyEventPollInterval),
					slog.Int("batch_size", organizationAPIKeyEventBatchSize),
					slog.Int("max_attempts", organizationAPIKeyEventMaxAttempts),
				)
				return nil
			},
			OnStop: func(ctx context.Context) error {
				logger.InfoContext(ctx, "stopping organization api key event worker")
				if cancel != nil {
					cancel()
				}
				return nil
			},
		},
	)
}

func (s *organizationAPIKeyEventService) ProcessQueueJob(
	ctx context.Context, job queue.Job,
) error {
	var payload organizationAPIKeyEventPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return queue.NonRetryable(
			fmt.Errorf(
				"invalid organization api key event payload: %w", err,
			),
		)
	}
	if payload.EventType == "" {
		return queue.NonRetryable(errors.New("organization api key event payload missing event_type"))
	}
	if payload.OrganizationID == "" {
		return queue.NonRetryable(errors.New("organization api key event payload missing organization_id"))
	}
	if payload.APIKeyID == "" {
		return queue.NonRetryable(errors.New("organization api key event payload missing api_key_id"))
	}

	switch payload.EventType {
	case organizationAPIKeyEventTypeExpiration:
		return s.processOrganizationAPIKeyExpiration(ctx, payload)
	default:
		return queue.NonRetryable(
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

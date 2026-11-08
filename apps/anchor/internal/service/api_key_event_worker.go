package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"anchor/internal/queueworker"

	"github.com/nanostack-dev/pgkit/queue"
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

// APIKeyEventWorker delivers the organization API key events.
type APIKeyEventWorker struct {
	queueworker.Worker
}

func NewAPIKeyEventWorker(
	eventService OrganizationAPIKeyEventService, queueClient *queue.Client, logger *slog.Logger,
) (*APIKeyEventWorker, error) {
	logger = logger.With(slog.String("component", "organization_api_key_event_worker"))

	registry := queue.NewHandlerRegistry()
	if err := registry.Register(organizationAPIKeyEventQueueName, eventService.ProcessQueueJob); err != nil {
		return nil, fmt.Errorf("register organization api key event queue handler: %w", err)
	}

	worker, err := queue.NewWorker(
		queueClient, registry, queue.WorkerConfig{
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
		return nil, fmt.Errorf("initialize organization api key event worker: %w", err)
	}

	return &APIKeyEventWorker{queueworker.New(worker, logger, queueworker.Spec{
		Started: "organization api key event worker started",
		StartedAttrs: []slog.Attr{
			slog.String("queue_name", organizationAPIKeyEventQueueName),
			slog.Duration("poll_interval", organizationAPIKeyEventPollInterval),
			slog.Int("batch_size", organizationAPIKeyEventBatchSize),
			slog.Int("max_attempts", organizationAPIKeyEventMaxAttempts),
		},
		Failed:   "organization api key event worker stopped with error",
		Stopping: "stopping organization api key event worker",
	})}, nil
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

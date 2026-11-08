package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"anchor/internal/queueworker"

	"github.com/nanostack-dev/pgkit/queue"
)

const (
	licenseTemplateSyncWorkerID     = "anchor-license-template-sync-worker"
	licenseTemplateSyncPollInterval = 1 * time.Second
	licenseTemplateSyncReapInterval = 30 * time.Second
	licenseTemplateSyncVisibility   = 1 * time.Minute
	licenseTemplateSyncWorkerBatch  = 10
	licenseTemplateSyncBackoffBase  = 1 * time.Second
	licenseTemplateSyncBackoffMax   = 5 * time.Minute
)

// LicenseTemplateSyncWorker propagates template value updates onto the
// licenses that follow the template.
type LicenseTemplateSyncWorker struct {
	queueworker.Worker
}

func NewLicenseTemplateSyncWorker(
	sync LicenseTemplateSyncService, queueClient *queue.Client, logger *slog.Logger,
) (*LicenseTemplateSyncWorker, error) {
	logger = logger.With(slog.String("component", "license_template_sync_worker"))

	registry := queue.NewHandlerRegistry()
	if err := registry.Register(licenseTemplateSyncQueueName, sync.ProcessQueueJob); err != nil {
		return nil, fmt.Errorf("register license template sync queue handler: %w", err)
	}

	worker, err := queue.NewWorker(
		queueClient, registry, queue.WorkerConfig{
			WorkerID:          licenseTemplateSyncWorkerID,
			PollInterval:      licenseTemplateSyncPollInterval,
			ReapInterval:      licenseTemplateSyncReapInterval,
			VisibilityTimeout: licenseTemplateSyncVisibility,
			BatchSizePerQueue: licenseTemplateSyncWorkerBatch,
			BackoffBase:       licenseTemplateSyncBackoffBase,
			BackoffMax:        licenseTemplateSyncBackoffMax,
			OnJobFailed: func(ctx context.Context, job queue.Job) {
				logger.ErrorContext(ctx, "license template sync job permanently failed",
					slog.Int64("job_id", job.ID),
					slog.String("queue", job.QueueName),
					slog.Int("attempts", job.Attempts),
					slog.String("last_error", job.LastError.String),
				)
			},
			OnJobStuck: func(ctx context.Context, result queue.ReapResult) {
				logger.WarnContext(ctx, "license template sync jobs stuck in processing were reaped",
					slog.Int64("requeued", result.Requeued),
					slog.Int64("failed", result.Failed),
				)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("initialize license template sync worker: %w", err)
	}

	return &LicenseTemplateSyncWorker{queueworker.New(worker, logger, queueworker.Spec{
		Started: "license template sync worker started",
		StartedAttrs: []slog.Attr{
			slog.String("queue_name", licenseTemplateSyncQueueName),
			slog.Duration("poll_interval", licenseTemplateSyncPollInterval),
		},
		Failed:   "license template sync worker stopped with error",
		Stopping: "stopping license template sync worker",
	})}, nil
}

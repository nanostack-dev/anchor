package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/pgkit/queue"
	"go.uber.org/fx"
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

type LicenseTemplateSyncWorkerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Sync      LicenseTemplateSyncService
	Queue     *queue.Client
	Logger    *slog.Logger
}

func RegisterLicenseTemplateSyncWorker(p LicenseTemplateSyncWorkerParams) {
	logger := p.Logger.With(slog.String("component", "license_template_sync_worker"))

	registry := queue.NewHandlerRegistry()
	if err := registry.Register(licenseTemplateSyncQueueName, p.Sync.ProcessQueueJob); err != nil {
		logger.Error("failed to register license template sync queue handler", log.Err(err))
		return
	}

	worker, err := queue.NewWorker(
		p.Queue, registry, queue.WorkerConfig{
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
		logger.Error("failed to initialize license template sync worker", log.Err(err))
		return
	}

	var cancel context.CancelFunc

	p.Lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				workerCtx, workerCancel := context.WithCancel(context.Background())
				cancel = workerCancel

				go func() {
					defer workerCancel()
					if runErr := worker.Run(workerCtx); runErr != nil {
						logger.ErrorContext(
							workerCtx,
							"license template sync worker stopped with error",
							log.Err(runErr),
						)
					}
				}()

				logger.InfoContext(ctx, "license template sync worker started",
					slog.String("queue_name", licenseTemplateSyncQueueName),
					slog.Duration("poll_interval", licenseTemplateSyncPollInterval),
				)
				return nil
			},
			OnStop: func(ctx context.Context) error {
				logger.InfoContext(ctx, "stopping license template sync worker")
				if cancel != nil {
					cancel()
				}
				return nil
			},
		},
	)
}

package service

import (
	"context"
	"time"

	"github.com/nanostack-dev/pgkit/queue"
	"github.com/rs/zerolog"
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
	Logger    zerolog.Logger
}

// RegisterLicenseTemplateSyncWorker starts the pgkit queue worker that
// propagates a template value update onto the licenses naming it, mirroring
// RegisterAPIKeyEventWorker.
func RegisterLicenseTemplateSyncWorker(p LicenseTemplateSyncWorkerParams) {
	logger := p.Logger.With().Str("component", "license_template_sync_worker").Logger()

	registry := queue.NewHandlerRegistry()
	if err := registry.Register(
		licenseTemplateSyncQueueName, func(ctx context.Context, job queue.Job) error {
			return p.Sync.ProcessQueueJob(ctx, job)
		},
	); err != nil {
		logger.Error().Err(err).Msg("failed to register license template sync queue handler")
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
			OnJobFailed: func(_ context.Context, job queue.Job) {
				logger.Error().
					Int64("job_id", job.ID).
					Str("queue", job.QueueName).
					Int("attempts", job.Attempts).
					Str("last_error", job.LastError.String).
					Msg("license template sync job permanently failed")
			},
			OnJobStuck: func(_ context.Context, result queue.ReapResult) {
				logger.Warn().
					Int64("requeued", result.Requeued).
					Int64("failed", result.Failed).
					Msg("license template sync jobs stuck in processing were reaped")
			},
		},
	)
	if err != nil {
		logger.Error().Err(err).Msg("failed to initialize license template sync worker")
		return
	}

	var cancel context.CancelFunc

	p.Lifecycle.Append(
		fx.Hook{
			OnStart: func(_ context.Context) error {
				workerCtx, workerCancel := newLicenseTemplateSyncWorkerContext()
				cancel = workerCancel

				go func() {
					if runErr := worker.Run(workerCtx); runErr != nil {
						logger.Error().Err(runErr).Msg("license template sync worker stopped with error")
					}
				}()

				logger.Info().
					Str("queue_name", licenseTemplateSyncQueueName).
					Dur("poll_interval", licenseTemplateSyncPollInterval).
					Msg("license template sync worker started")
				return nil
			},
			OnStop: func(_ context.Context) error {
				logger.Info().Msg("stopping license template sync worker")
				if cancel != nil {
					cancel()
				}
				return nil
			},
		},
	)
}

func newLicenseTemplateSyncWorkerContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

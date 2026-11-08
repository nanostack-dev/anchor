package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	domainintegration "anchor/internal/domain/integration"
	"anchor/internal/integration/provider"
	"anchor/internal/repository"
	serviceconfig "anchor/internal/service/config"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/nanostack-dev/pgkit/queue"

	"anchor/internal/queueworker"
)

// Hardcoded integration event worker settings.
const (
	integrationQueueName          = "integration-webhooks"
	integrationReconcileQueueName = "integration-reconcile"
	integrationWorkerID           = "anchor-integration-worker"
	integrationPollInterval       = 2 * time.Second
	integrationReapInterval       = 30 * time.Second
	integrationVisibilityTimeout  = 2 * time.Minute
	integrationBatchSize          = 50
	integrationMaxAttempts        = 6
	integrationBackoffBase        = 1 * time.Second
	integrationBackoffMax         = 5 * time.Minute

	defaultIntegrationReconcileScheduleEvery = 15 * time.Minute

	// lockKeyReconcileSchedulerSeed is the advisory lock key used to ensure only one
	// replica seeds the initial reconcile scheduler job at startup.
	lockKeyReconcileSchedulerSeed = "integration.reconcile_scheduler.seed"

	lockKeyInstanceReconcilePrefix = "integration.reconcile.instance:"
)

// IntegrationEventWorker runs the integration queue worker. Its Start seeds
// the first reconcile scheduler job, before the App serves.
type IntegrationEventWorker struct {
	queueworker.Worker

	lock             *pglock.Client
	queue            *queue.Client
	instanceRepo     repository.IntegrationInstanceRepository
	registry         *provider.Registry
	scheduleInterval time.Duration
	logger           *slog.Logger
}

// NewIntegrationEventWorker builds the pgkit queue worker runtime.
func NewIntegrationEventWorker(
	integrationService IntegrationService,
	queueClient *queue.Client,
	lock *pglock.Client,
	instanceRepo repository.IntegrationInstanceRepository,
	registry *provider.Registry,
	logger *slog.Logger,
	coreConfig *serviceconfig.CoreConfig,
) (*IntegrationEventWorker, error) {
	logger = logger.With(slog.String("component", "integration_event_worker"))

	scheduleInterval := parseScheduleInterval(logger, coreConfig)

	handlers := queue.NewHandlerRegistry()
	if err := registerQueueHandlers(handlers, integrationService, logger); err != nil {
		return nil, err
	}

	worker, err := queue.NewWorker(queueClient, handlers, queue.WorkerConfig{
		WorkerID:          integrationWorkerID,
		PollInterval:      integrationPollInterval,
		ReapInterval:      integrationReapInterval,
		VisibilityTimeout: integrationVisibilityTimeout,
		BatchSizePerQueue: integrationBatchSize,
		BackoffBase:       integrationBackoffBase,
		BackoffMax:        integrationBackoffMax,
		OnJobFailed: func(ctx context.Context, job queue.Job) {
			logger.ErrorContext(ctx, "queue job permanently failed",
				slog.Int64("job_id", job.ID),
				slog.String("queue", job.QueueName),
				slog.Int("attempts", job.Attempts),
				slog.String("last_error", job.LastError.String),
			)
		},
		OnJobStuck: func(ctx context.Context, result queue.ReapResult) {
			logger.LogAttrs(ctx, reapLogLevel(result), "queue jobs stuck in processing were reaped",
				slog.Int64("requeued", result.Requeued),
				slog.Int64("failed", result.Failed),
			)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("initialize integration event worker: %w", err)
	}

	return &IntegrationEventWorker{
		Worker: queueworker.New(worker, logger, queueworker.Spec{
			Started: "integration queue worker started",
			StartedAttrs: []slog.Attr{
				slog.String("queue_name", integrationQueueName),
				slog.Duration("poll_interval", integrationPollInterval),
				slog.Duration("reconcile_schedule_interval", scheduleInterval),
				slog.Int("batch_size", integrationBatchSize),
				slog.Int("max_attempts", integrationMaxAttempts),
			},
			Failed:   "integration queue worker stopped with error",
			Stopping: "stopping integration queue worker",
		}),
		lock:             lock,
		queue:            queueClient,
		instanceRepo:     instanceRepo,
		registry:         registry,
		scheduleInterval: scheduleInterval,
		logger:           logger,
	}, nil
}

// Start seeds the initial reconcile scheduler job, using a distributed
// advisory lock so that only one replica inserts the first job even in
// multi-instance deployments. A failure is logged and does not fail the
// start.
func (w *IntegrationEventWorker) Start(ctx context.Context) error {
	seedReconcileScheduler(ctx, w.lock, w.queue, w.instanceRepo, w.registry, w.scheduleInterval, w.logger)
	return nil
}

// registerQueueHandlers wires all queue handler functions into the registry.
// Returns non-nil error if any registration fails (already logs the error).
func registerQueueHandlers(
	registry *queue.HandlerRegistry,
	svc IntegrationService,
	logger *slog.Logger,
) error {
	if err := registry.Register(integrationQueueName, func(ctx context.Context, job queue.Job) error {
		return svc.ProcessQueueJob(ctx, job)
	}); err != nil {
		logger.Error("failed to register integration queue handler", log.Err(err))
		return err
	}

	if err := registry.Register(integrationReconcileQueueName, func(ctx context.Context, job queue.Job) error {
		return svc.ProcessReconcileQueueJob(ctx, job)
	}); err != nil {
		logger.Error("failed to register integration reconcile queue handler", log.Err(err))
		return err
	}

	return nil
}

// parseScheduleInterval returns the configured reconcile scheduler interval,
// or the default when it is not positive.
func parseScheduleInterval(logger *slog.Logger, cfg *serviceconfig.CoreConfig) time.Duration {
	if cfg == nil {
		return defaultIntegrationReconcileScheduleEvery
	}

	d := cfg.Integration.ReconcileScheduleInterval
	if d <= 0 {
		logger.Warn("invalid integration reconcile scheduler interval, using default",
			slog.Duration("interval", d),
			slog.Duration("fallback_interval", defaultIntegrationReconcileScheduleEvery),
		)
		return defaultIntegrationReconcileScheduleEvery
	}

	return d
}

// hasPendingOrProcessingSchedulerJob checks if a scheduler job is already in the queue.
func hasPendingOrProcessingSchedulerJob(ctx context.Context, queueClient *queue.Client) (bool, error) {
	const maxSchedulerJobsToInspect = 1000
	for _, status := range []queue.JobStatus{queue.StatusPending, queue.StatusProcessing} {
		jobs, err := queueClient.ListJobs(ctx, queue.ListJobsParams{
			QueueName: integrationReconcileQueueName,
			Status:    status,
			Limit:     maxSchedulerJobsToInspect,
		})
		if err != nil {
			return false, err
		}
		for _, job := range jobs {
			var payload integrationReconcileQueuePayload
			if jsonErr := json.Unmarshal(job.Payload, &payload); jsonErr == nil && payload.IsScheduler {
				return true, nil
			}
		}
	}
	return false, nil
}

// seedReconcileScheduler uses a DB advisory lock to ensure a single replica seeds the
// initial recurring reconcile scheduler job. It only seeds when the Clerk provider is
// registered AND at least one Clerk instance already has an API key. If the lock is
// already held by another replica (concurrent startup), the seed is safely skipped.
func seedReconcileScheduler(
	ctx context.Context,
	lock *pglock.Client,
	queueClient *queue.Client,
	instanceRepo repository.IntegrationInstanceRepository,
	registry *provider.Registry,
	scheduleInterval time.Duration,
	logger *slog.Logger,
) {
	// Only seed when the Clerk provider is registered.
	if _, err := registry.GetProvider(string(domainintegration.ProviderTypeClerk)); err != nil {
		logger.DebugContext(ctx, "Clerk provider not registered; skipping reconcile scheduler seed")
		return
	}

	acquired, err := lock.TryWithLock(ctx, lockKeyReconcileSchedulerSeed, func(ctx context.Context, _ *sql.Tx) error {
		// Inside the lock: check whether at least one Clerk instance has an API key.
		instances, listErr := instanceRepo.ListByProviderInternal(
			ctx, string(domainintegration.ProviderTypeClerk),
		)
		if listErr != nil {
			return listErr
		}

		hasKey := functional.Slice(instances).AnyMatch(func(inst domainintegration.Instance) bool {
			return hasClerkAPIKey(inst.ConfigJSON)
		})

		if !hasKey {
			logger.DebugContext(ctx, "no Clerk instances with API key found; skipping reconcile scheduler seed")
			return nil
		}

		// Check if a scheduler is already queued or processing.
		hasScheduler, err := hasPendingOrProcessingSchedulerJob(ctx, queueClient)
		if err != nil {
			return err
		}
		if hasScheduler {
			return nil // Already queued or processing
		}

		return enqueueReconcileSchedulerJob(ctx, queueClient, nil)
	})
	if err != nil {
		logger.ErrorContext(ctx, "failed to seed initial reconcile scheduler job", log.Err(err))
		return
	}
	if !acquired {
		logger.DebugContext(ctx, "reconcile scheduler seed lock not acquired, another replica is seeding")
		return
	}

	logger.InfoContext(ctx, "seeded initial reconcile scheduler job",
		slog.Duration("schedule_interval", scheduleInterval),
	)
}

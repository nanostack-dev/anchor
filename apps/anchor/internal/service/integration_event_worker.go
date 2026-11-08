package service

import (
	"context"
	"database/sql"
	"encoding/json"
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

	"go.uber.org/fx"
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

// IntegrationEventWorkerParams groups the dependencies for the async event worker.
type IntegrationEventWorkerParams struct {
	fx.In
	Lifecycle          fx.Lifecycle
	IntegrationService IntegrationService
	Queue              *queue.Client
	Lock               *pglock.Client
	InstanceRepo       repository.IntegrationInstanceRepository
	Registry           *provider.Registry
	Logger             *slog.Logger
	CoreConfig         *serviceconfig.CoreConfig
}

// RegisterIntegrationEventWorker starts the pgkit queue worker runtime.
func RegisterIntegrationEventWorker(p IntegrationEventWorkerParams) {
	logger := p.Logger.With(slog.String("component", "integration_event_worker"))

	scheduleInterval := parseScheduleInterval(logger, p.CoreConfig)

	registry := queue.NewHandlerRegistry()
	if err := registerQueueHandlers(registry, p.IntegrationService, logger); err != nil {
		return
	}

	worker, err := queue.NewWorker(p.Queue, registry, queue.WorkerConfig{
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
		logger.Error("failed to initialize integration event worker", log.Err(err))
		return
	}

	var cancel context.CancelFunc

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			var ctx context.Context

			ctx, cancel = context.WithCancel(context.Background()) // #nosec G118 -- canceled in lifecycle OnStop

			go func() {
				if runErr := worker.Run(ctx); runErr != nil {
					logger.ErrorContext(ctx, "integration queue worker stopped with error", log.Err(runErr))
				}
			}()

			// Seed the initial reconcile scheduler job using a distributed advisory lock so
			// that only one replica inserts the first job even in multi-instance deployments.
			seedReconcileScheduler(ctx, p.Lock, p.Queue, p.InstanceRepo, p.Registry, scheduleInterval, logger)

			logger.InfoContext(ctx, "integration queue worker started",
				slog.String("queue_name", integrationQueueName),
				slog.Duration("poll_interval", integrationPollInterval),
				slog.Duration("reconcile_schedule_interval", scheduleInterval),
				slog.Int("batch_size", integrationBatchSize),
				slog.Int("max_attempts", integrationMaxAttempts),
			)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.InfoContext(ctx, "stopping integration queue worker")
			if cancel != nil {
				cancel()
			}
			return nil
		},
	})
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

// parseScheduleInterval parses the configured reconcile scheduler interval or falls back to the default.
func parseScheduleInterval(logger *slog.Logger, cfg *serviceconfig.CoreConfig) time.Duration {
	if cfg == nil {
		return defaultIntegrationReconcileScheduleEvery
	}

	raw := cfg.Integration.ReconcileScheduleInterval
	if raw == "" {
		return defaultIntegrationReconcileScheduleEvery
	}

	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		logger.Warn("invalid integration reconcile scheduler interval, using default",
			log.Err(err),
			slog.String("interval", raw),
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

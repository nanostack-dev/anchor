package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	domainintegration "anchor/internal/domain/integration"
	"anchor/internal/integration/provider"
	"anchor/internal/repository"
	serviceconfig "anchor/internal/service/config"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/nanostack-dev/pgkit/queue"

	"github.com/rs/zerolog"
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
	Logger             zerolog.Logger
	CoreConfig         *serviceconfig.CoreConfig
}

// RegisterIntegrationEventWorker starts the pgkit queue worker runtime.
func RegisterIntegrationEventWorker(p IntegrationEventWorkerParams) {
	logger := p.Logger.With().
		Str("component", "integration_event_worker").
		Logger()

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
		OnJobFailed: func(_ context.Context, job queue.Job) {
			logger.Error().
				Int64("job_id", job.ID).
				Str("queue", job.QueueName).
				Int("attempts", job.Attempts).
				Str("last_error", job.LastError.String).
				Msg("queue job permanently failed")
		},
		OnJobStuck: func(_ context.Context, result queue.ReapResult) {
			logger.WithLevel(reapLogLevel(result)).
				Int64("requeued", result.Requeued).
				Int64("failed", result.Failed).
				Msg("queue jobs stuck in processing were reaped")
		},
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to initialize integration event worker")
		return
	}

	var cancel context.CancelFunc

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			var ctx context.Context

			ctx, cancel = context.WithCancel(context.Background()) // #nosec G118 -- canceled in lifecycle OnStop

			go func() {
				if runErr := worker.Run(ctx); runErr != nil {
					logger.Error().Err(runErr).Msg("integration queue worker stopped with error")
				}
			}()

			// Seed the initial reconcile scheduler job using a distributed advisory lock so
			// that only one replica inserts the first job even in multi-instance deployments.
			seedReconcileScheduler(ctx, p.Lock, p.Queue, p.InstanceRepo, p.Registry, scheduleInterval, logger)

			logger.Info().
				Str("queue_name", integrationQueueName).
				Dur("poll_interval", integrationPollInterval).
				Dur("reconcile_schedule_interval", scheduleInterval).
				Int("batch_size", integrationBatchSize).
				Int("max_attempts", integrationMaxAttempts).
				Msg("integration queue worker started")
			return nil
		},
		OnStop: func(_ context.Context) error {
			logger.Info().Msg("stopping integration queue worker")
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
	logger zerolog.Logger,
) error {
	if err := registry.Register(integrationQueueName, func(ctx context.Context, job queue.Job) error {
		return svc.ProcessQueueJob(ctx, job)
	}); err != nil {
		logger.Error().Err(err).Msg("failed to register integration queue handler")
		return err
	}

	if err := registry.Register(integrationReconcileQueueName, func(ctx context.Context, job queue.Job) error {
		return svc.ProcessReconcileQueueJob(ctx, job)
	}); err != nil {
		logger.Error().Err(err).Msg("failed to register integration reconcile queue handler")
		return err
	}

	return nil
}

// parseScheduleInterval parses the configured reconcile scheduler interval or falls back to the default.
func parseScheduleInterval(logger zerolog.Logger, cfg *serviceconfig.CoreConfig) time.Duration {
	if cfg == nil {
		return defaultIntegrationReconcileScheduleEvery
	}

	raw := cfg.Integration.ReconcileScheduleInterval
	if raw == "" {
		return defaultIntegrationReconcileScheduleEvery
	}

	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		logger.Warn().Err(err).
			Str("interval", raw).
			Dur("fallback_interval", defaultIntegrationReconcileScheduleEvery).
			Msg("invalid integration reconcile scheduler interval, using default")
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
	logger zerolog.Logger,
) {
	// Only seed when the Clerk provider is registered.
	if _, err := registry.GetProvider(string(domainintegration.ProviderTypeClerk)); err != nil {
		logger.Debug().Msg("Clerk provider not registered; skipping reconcile scheduler seed")
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
			logger.Debug().Msg("no Clerk instances with API key found; skipping reconcile scheduler seed")
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
		logger.Error().Err(err).Msg("failed to seed initial reconcile scheduler job")
		return
	}
	if !acquired {
		logger.Debug().Msg("reconcile scheduler seed lock not acquired, another replica is seeding")
		return
	}

	logger.Info().
		Dur("schedule_interval", scheduleInterval).
		Msg("seeded initial reconcile scheduler job")
}

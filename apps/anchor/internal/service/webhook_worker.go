package service

import (
	"context"
	"time"

	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"anchor/internal/domain/webhook"
)

const (
	webhookFanoutWorkerID  = "anchor-webhook-fanout-worker"
	webhookDeliverWorkerID = "anchor-webhook-deliver-worker"

	webhookPollInterval = 2 * time.Second
	webhookReapInterval = 30 * time.Second

	// webhookFanoutVisibility is short: fan-out only touches the database.
	webhookFanoutVisibility = 1 * time.Minute
	// webhookDeliverVisibility comfortably exceeds the 15s request timeout, so
	// the reaper never steals a job from a worker that is still waiting on a
	// slow receiver.
	webhookDeliverVisibility = 2 * time.Minute

	webhookFanoutBatchSize  = 25
	webhookDeliverBatchSize = 25

	webhookBackoffBase = 5 * time.Second
	webhookBackoffMax  = 5 * time.Minute
)

type WebhookWorkerParams struct {
	fx.In
	Lifecycle       fx.Lifecycle
	FanoutService   WebhookFanoutService
	DeliveryService WebhookDeliveryService
	Queue           *pgqueue.Client
	Logger          zerolog.Logger
}

// RegisterWebhookWorkers starts one worker per queue.
//
// Two workers rather than one: pgqueue's retry schedule is per-worker, and
// delivery needs the jittered eight-rung ladder while fan-out only needs a
// short database-failure backoff. Splitting them also keeps a single
// unreachable endpoint from occupying fan-out capacity.
func RegisterWebhookWorkers(p WebhookWorkerParams) {
	logger := p.Logger.With().Str("component", "webhook_worker").Logger()

	startWebhookWorker(p, logger, webhookWorkerSpec{
		queueName:  WebhookFanoutQueueName,
		workerID:   webhookFanoutWorkerID,
		visibility: webhookFanoutVisibility,
		batchSize:  webhookFanoutBatchSize,
		handler: func(ctx context.Context, job pgqueue.Job) error {
			return p.FanoutService.ProcessQueueJob(ctx, job)
		},
	})

	startWebhookWorker(p, logger, webhookWorkerSpec{
		queueName:  WebhookDeliverQueueName,
		workerID:   webhookDeliverWorkerID,
		visibility: webhookDeliverVisibility,
		batchSize:  webhookDeliverBatchSize,
		handler: func(ctx context.Context, job pgqueue.Job) error {
			return p.DeliveryService.ProcessQueueJob(ctx, job)
		},
		// The ladder is supplied to pgqueue rather than reimplemented: claim,
		// lease, visibility timeout, stuck-job reaping and attempt counting
		// already exist there.
		retryDelay: func(job pgqueue.Job, err error) time.Duration {
			// A receiver that asked for a specific pause via Retry-After outranks
			// our ladder; everything else rides the jittered schedule.
			if delay, ok := RetryAfterDelay(err); ok {
				return delay
			}

			return webhook.RetryDelay(job.Attempts, webhook.DefaultJitterer())
		},
	})
}

type webhookWorkerSpec struct {
	queueName  string
	workerID   string
	visibility time.Duration
	batchSize  int
	handler    func(ctx context.Context, job pgqueue.Job) error
	retryDelay pgqueue.RetryDelayFunc
}

func startWebhookWorker(p WebhookWorkerParams, logger zerolog.Logger, spec webhookWorkerSpec) {
	workerLogger := logger.With().Str("queue", spec.queueName).Logger()

	registry := pgqueue.NewHandlerRegistry()
	if err := registry.Register(spec.queueName, spec.handler); err != nil {
		workerLogger.Error().Err(err).Msg("failed to register webhook queue handler")
		return
	}

	worker, err := pgqueue.NewWorker(
		p.Queue, registry, pgqueue.WorkerConfig{
			WorkerID:          spec.workerID,
			PollInterval:      webhookPollInterval,
			ReapInterval:      webhookReapInterval,
			VisibilityTimeout: spec.visibility,
			BatchSizePerQueue: spec.batchSize,
			BackoffBase:       webhookBackoffBase,
			BackoffMax:        webhookBackoffMax,
			RetryDelay:        spec.retryDelay,
			OnJobFailed: func(_ context.Context, job pgqueue.Job) {
				workerLogger.Error().
					Int64("job_id", job.ID).
					Int("attempts", job.Attempts).
					Str("last_error", job.LastError.String).
					Msg("webhook job permanently failed")
			},
			OnJobStuck: func(_ context.Context, result pgqueue.ReapResult) {
				workerLogger.WithLevel(reapLogLevel(result)).
					Int64("requeued", result.Requeued).
					Int64("failed", result.Failed).
					Msg("webhook jobs stuck in processing were reaped")
			},
		},
	)
	if err != nil {
		workerLogger.Error().Err(err).Msg("failed to initialize webhook worker")
		return
	}

	var cancel context.CancelFunc

	p.Lifecycle.Append(
		fx.Hook{
			OnStart: func(_ context.Context) error {
				workerCtx, workerCancel := newWebhookWorkerContext()
				cancel = workerCancel

				go func() {
					if runErr := worker.Run(workerCtx); runErr != nil {
						workerLogger.Error().Err(runErr).Msg("webhook worker stopped with error")
					}
				}()

				workerLogger.Info().
					Dur("poll_interval", webhookPollInterval).
					Int("batch_size", spec.batchSize).
					Msg("webhook worker started")

				return nil
			},
			OnStop: func(_ context.Context) error {
				workerLogger.Info().Msg("stopping webhook worker")
				if cancel != nil {
					cancel()
				}

				return nil
			},
		},
	)
}

// newWebhookWorkerContext returns the long-lived worker context. The cancel
// function is stored on the lifecycle hook and invoked from OnStop.
func newWebhookWorkerContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

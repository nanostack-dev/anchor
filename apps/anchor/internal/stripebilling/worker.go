package stripebilling

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/nanostack-dev/pgkit/queue"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const (
	EventsQueueName       = "stripe-billing-events"
	ReconcileQueueName    = "stripe-billing-reconcile"
	workerID              = "anchor-stripe-billing-worker"
	workerPollInterval    = time.Second
	workerReapInterval    = 30 * time.Second
	workerVisibility      = 10 * time.Minute
	workerBatchSize       = 1
	workerMaxAttempts     = 6
	workerBackoffBase     = time.Second
	workerBackoffMax      = 5 * time.Minute
	reconcileInterval     = time.Minute
	schedulerLockKey      = "stripe-billing.reconcile-scheduler"
	schedulerInspectLimit = 1
)

type EventQueuePayload struct {
	IntegrationInstanceID string `json:"integration_instance_id"`
}

type WorkerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Manager   *Manager
	Queue     *queue.Client
	Lock      *pglock.Client
	Logger    zerolog.Logger
}

type billingProcessor interface {
	RunInternal(context.Context, string) error
	ReconcileInternal(context.Context) error
}

type schedulerQueue interface {
	ListJobs(context.Context, queue.ListJobsParams) ([]queue.Job, error)
	EnqueueTx(context.Context, *sql.Tx, queue.EnqueueParams) (int64, error)
}

type schedulerLock interface {
	TryWithLock(context.Context, string, func(context.Context, *sql.Tx) error) (bool, error)
}

type reconcileScheduler struct {
	queue  schedulerQueue
	lock   schedulerLock
	logger zerolog.Logger
}

type billingJobHandler struct {
	processor billingProcessor
	scheduler reconcileScheduler
}

func RegisterWorker(p WorkerParams) error {
	logger := p.Logger.With().Str("component", "stripe_billing_worker").Logger()
	scheduler := reconcileScheduler{queue: p.Queue, lock: p.Lock, logger: logger}
	registry := queue.NewHandlerRegistry()
	if err := registerHandlers(registry, p.Manager, scheduler); err != nil {
		return err
	}
	worker, err := queue.NewWorker(p.Queue, registry, queue.WorkerConfig{
		WorkerID:          workerID,
		PollInterval:      workerPollInterval,
		ReapInterval:      workerReapInterval,
		VisibilityTimeout: workerVisibility,
		BatchSizePerQueue: workerBatchSize,
		BackoffBase:       workerBackoffBase,
		BackoffMax:        workerBackoffMax,
		OnJobFailed: func(_ context.Context, job queue.Job) {
			logger.Error().
				Int64("job_id", job.ID).
				Str("queue", job.QueueName).
				Int("attempts", job.Attempts).
				Msg("Stripe billing queue job permanently failed")
		},
	})
	if err != nil {
		return fmt.Errorf("initialize Stripe billing worker: %w", err)
	}

	var cancel context.CancelFunc
	done := make(chan error, 1)
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if seedErr := scheduler.schedule(ctx, true); seedErr != nil {
				return seedErr
			}
			workerCtx, workerCancel := context.WithCancel( //nolint:gosec // canceled and joined in OnStop.
				context.Background(),
			)
			cancel = workerCancel
			go func() {
				done <- worker.Run(workerCtx)
			}()
			logger.Info().Str("queue", EventsQueueName).Msg("Stripe billing worker started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if cancel == nil {
				return nil
			}
			cancel()
			select {
			case runErr := <-done:
				if errors.Is(runErr, context.Canceled) {
					return nil
				}
				return runErr
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	return nil
}

func registerHandlers(
	registry *queue.HandlerRegistry,
	processor billingProcessor,
	scheduler reconcileScheduler,
) error {
	handler := billingJobHandler{processor: processor, scheduler: scheduler}
	if err := registry.Register(EventsQueueName, handler.event); err != nil {
		return err
	}
	return registry.Register(ReconcileQueueName, handler.reconcile)
}

func (h billingJobHandler) event(ctx context.Context, job queue.Job) error {
	var payload EventQueuePayload
	if decodeErr := json.Unmarshal(job.Payload, &payload); decodeErr != nil {
		return queue.NonRetryable(fmt.Errorf("invalid Stripe billing queue payload: %w", decodeErr))
	}
	if payload.IntegrationInstanceID == "" {
		return queue.NonRetryable(errors.New("stripe billing queue payload missing integration_instance_id"))
	}
	return h.processor.RunInternal(ctx, payload.IntegrationInstanceID)
}

func (h billingJobHandler) reconcile(ctx context.Context, _ queue.Job) error {
	if reconcileErr := h.processor.ReconcileInternal(ctx); reconcileErr != nil {
		h.scheduler.logger.Error().Err(reconcileErr).Msg("Stripe billing reconciliation failed")
	}
	return h.scheduler.schedule(ctx, false)
}

func (s reconcileScheduler) schedule(ctx context.Context, seed bool) error {
	acquired, err := s.lock.TryWithLock(ctx, schedulerLockKey, func(lockCtx context.Context, tx *sql.Tx) error {
		statuses := []queue.JobStatus{queue.StatusPending}
		if seed {
			statuses = append(statuses, queue.StatusProcessing)
		}
		for _, status := range statuses {
			jobs, listErr := s.queue.ListJobs(lockCtx, queue.ListJobsParams{
				QueueName: ReconcileQueueName,
				Status:    status,
				Limit:     schedulerInspectLimit,
			})
			if listErr != nil {
				return listErr
			}
			if len(jobs) > 0 {
				return nil
			}
		}
		nextAt := time.Now().Add(reconcileInterval)
		_, enqueueErr := s.queue.EnqueueTx(lockCtx, tx, queue.EnqueueParams{
			QueueName:   ReconcileQueueName,
			Payload:     json.RawMessage(`{}`),
			AvailableAt: &nextAt,
			MaxAttempts: workerMaxAttempts,
		})
		return enqueueErr
	})
	if err != nil {
		return fmt.Errorf("schedule Stripe billing reconciliation: %w", err)
	}
	if !acquired && !seed {
		return errors.New("stripe billing reconciliation scheduler is busy")
	}
	return nil
}

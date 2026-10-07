package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nanostack-dev/pgkit/queue"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
	serviceconfig "anchor/internal/service/config"
	"anchor/internal/workflow/engine"
	"anchor/internal/workflow/repository"
)

const (
	QueueName             = workflow.EventQueueName
	workerID              = "anchor-product-workflows-worker"
	workerPollInterval    = 2 * time.Second
	workerPollIntervalDev = 100 * time.Millisecond
	workerReapInterval    = 30 * time.Second
	workerVisibility      = 5 * time.Minute
	workerBatchSize       = 20
	workerBackoffBase     = 1 * time.Second
	workerBackoffMax      = 5 * time.Minute
)

// Listener enqueues an event for the worker only when the product has an
// enabled workflow it would start, so most events cost one indexed read.
func Listener(repo repository.Repository) events.Listener {
	return events.Listener{
		QueueName: QueueName,
		Accepts: func(ctx context.Context, productID string, eventType events.Type) (bool, error) {
			return repo.HasEnabledForTriggerInternal(ctx, productID, string(eventType))
		},
	}
}

type WorkerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Queue     *queue.Client
	Repo      repository.Repository
	Runner    *Runner
	Logger    zerolog.Logger
	Core      *serviceconfig.CoreConfig
}

func RegisterWorker(p WorkerParams) error {
	logger := p.Logger.With().Str("component", "product_workflows_worker").Logger()
	handler := &eventHandler{repo: p.Repo, runner: p.Runner, logger: logger}

	registry := queue.NewHandlerRegistry()
	if err := registry.Register(QueueName, handler.handleJob); err != nil {
		return err
	}
	pollInterval := workerPollInterval
	if p.Core != nil && !p.Core.IsProduction() {
		pollInterval = workerPollIntervalDev
	}
	worker, err := queue.NewWorker(p.Queue, registry, queue.WorkerConfig{
		WorkerID:          workerID,
		PollInterval:      pollInterval,
		ReapInterval:      workerReapInterval,
		VisibilityTimeout: workerVisibility,
		BatchSizePerQueue: workerBatchSize,
		BackoffBase:       workerBackoffBase,
		BackoffMax:        workerBackoffMax,
		OnJobFailed: func(_ context.Context, job queue.Job) {
			logger.Error().
				Int64("job_id", job.ID).
				Int("attempts", job.Attempts).
				Str("last_error", job.LastError.String).
				Msg("product workflow trigger permanently failed")
		},
	})
	if err != nil {
		return err
	}

	var cancel context.CancelFunc
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			workerCtx, workerCancel := context.WithCancel(context.Background()) //nolint:gosec // canceled in OnStop
			cancel = workerCancel
			go func() {
				if runErr := worker.Run(workerCtx); runErr != nil {
					logger.Error().Err(runErr).Msg("product workflows worker stopped")
				}
			}()
			logger.Info().Str("queue_name", QueueName).Msg("product workflows worker started")
			return nil
		},
		OnStop: func(_ context.Context) error {
			if cancel != nil {
				cancel()
			}
			return nil
		},
	})
	return nil
}

type eventHandler struct {
	repo   repository.Repository
	runner *Runner
	logger zerolog.Logger
}

func (h *eventHandler) handleJob(ctx context.Context, job queue.Job) error {
	event, err := events.DecodeQueuedEvent(job.Payload)
	if err != nil {
		return err
	}
	workflows, err := h.repo.FindEnabledByTriggerInternal(ctx, event.ProductID, string(event.Type))
	if err != nil || len(workflows) == 0 {
		return err
	}
	data, err := event.Data()
	if err != nil {
		return err
	}
	occurredAt, err := event.OccurredAt()
	if err != nil {
		return err
	}
	var failures []error
	for _, wf := range workflows {
		if occurredAt.Before(wf.CreatedAt) {
			continue
		}
		execution := engine.Execution{
			Workflow:  wf,
			EventID:   event.EventID,
			EventType: string(event.Type),
			EventData: data,
			Trigger:   workflow.RunTriggerEvent,
			Depth:     event.Depth,
			Chain:     event.Chain,
		}
		var runErr error
		if reason := loopGuard(wf, event); reason != "" {
			runErr = h.runner.Prevent(ctx, execution, reason)
		} else {
			_, _, runErr = h.runner.Start(ctx, execution)
		}
		if runErr != nil {
			failures = append(failures, runErr)
		}
	}
	return errors.Join(failures...)
}

// loopGuard names why an event must not start the workflow: the workflow
// already ran earlier in the chain that produced the event, or the chain
// reached the maximum depth.
func loopGuard(wf workflow.Workflow, event events.QueuedEvent) string {
	if slices.Contains(event.Chain, wf.ID) {
		return "Loop prevented: this workflow already ran earlier in the chain that produced this event (" +
			strings.Join(append(slices.Clone(event.Chain), wf.ID), " → ") + ")."
	}
	if event.Depth >= workflow.MaxCausationDepth {
		return fmt.Sprintf("Chain stopped: %d workflow runs led to this event, the most one chain may hold.",
			event.Depth)
	}
	return ""
}

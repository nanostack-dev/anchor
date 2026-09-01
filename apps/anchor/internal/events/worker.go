package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	serviceconfig "anchor/internal/service/config"

	"github.com/nanostack-dev/pgkit/queue"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const (
	workerID              = "anchor-product-events-worker"
	workerPollInterval    = 2 * time.Second
	workerPollIntervalDev = 100 * time.Millisecond
	workerReapInterval    = 30 * time.Second
	workerVisibility      = 1 * time.Minute
	workerBatchSize       = 50
	workerBackoffBase     = 1 * time.Second
	workerBackoffMax      = 5 * time.Minute
	deliveryTimeout       = 15 * time.Second
	maxDeliveryBodyBytes  = 1 << 20
	successStatusMin      = 200
	successStatusMax      = 300
	goneStatus            = 410
)

type WorkerParams struct {
	fx.In
	Lifecycle       fx.Lifecycle
	Queue           *queue.Client
	EndpointService EndpointService
	Endpoints       EndpointRepository
	Logger          zerolog.Logger
	Core            *serviceconfig.CoreConfig
}

func RegisterWorker(p WorkerParams) {
	logger := p.Logger.With().Str("component", "product_events_worker").Logger()
	deliverer := &deliverer{
		endpoints: p.EndpointService,
		repo:      p.Endpoints,
		http: &http.Client{
			Timeout: deliveryTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		now:    time.Now,
		logger: logger,
	}

	registry := queue.NewHandlerRegistry()
	if err := registry.Register(queueName, deliverer.handleJob); err != nil {
		logger.Error().Err(err).Msg("failed to register product events queue handler")
		return
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
				Msg("product event delivery permanently failed")
		},
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to initialize product events worker")
		return
	}

	var cancel context.CancelFunc
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			workerCtx, workerCancel := context.WithCancel(context.Background()) //nolint:gosec // canceled in OnStop
			cancel = workerCancel
			go func() {
				if runErr := worker.Run(workerCtx); runErr != nil {
					logger.Error().Err(runErr).Msg("product events worker stopped")
				}
			}()
			logger.Info().Str("queue_name", queueName).Msg("product events worker started")
			return nil
		},
		OnStop: func(_ context.Context) error {
			if cancel != nil {
				cancel()
			}
			return nil
		},
	})
}

type deliverer struct {
	endpoints EndpointService
	repo      EndpointRepository
	http      *http.Client
	now       func() time.Time
	logger    zerolog.Logger
}

func (d *deliverer) handleJob(ctx context.Context, job queue.Job) error {
	var payload queuePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("events: decode job: %w", err)
	}
	target, found, err := d.endpoints.DeliveryTarget(ctx, payload.ProductID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	attemptAt := d.now()
	headers, err := Sign(target.Secret, payload.EventID, attemptAt, payload.Body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.URL, bytes.NewReader(payload.Body))
	if err != nil {
		return fmt.Errorf("events: build delivery request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := d.http.Do(req)
	if err != nil {
		return fmt.Errorf("events: deliver: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDeliveryBodyBytes))

	if resp.StatusCode >= successStatusMin && resp.StatusCode < successStatusMax {
		return nil
	}
	if resp.StatusCode == goneStatus {
		if delErr := d.repo.DeleteByProductIDInternal(ctx, payload.ProductID); delErr != nil {
			return fmt.Errorf("events: disable gone endpoint: %w", delErr)
		}
		return nil
	}
	d.logger.Warn().
		Int("status", resp.StatusCode).
		Str("event_id", payload.EventID).
		Msg("product event delivery rejected")
	return fmt.Errorf("events: delivery status %d", resp.StatusCode)
}

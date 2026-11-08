package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	serviceconfig "anchor/internal/service/config"
	"anchor/internal/telemetry"

	"github.com/nanostack-dev/pgkit/queue"
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
	Logger          *slog.Logger
	Core            *serviceconfig.CoreConfig
}

func RegisterWorker(p WorkerParams) {
	logger := p.Logger.With(slog.String("component", "product_events_worker"))
	deliverer := &deliverer{
		endpoints: p.EndpointService,
		repo:      p.Endpoints,
		http: &http.Client{
			Transport: telemetry.ExternalTransport(nil),
			Timeout:   deliveryTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		now:    time.Now,
		logger: logger,
	}

	registry := queue.NewHandlerRegistry()
	if err := registry.Register(queueName, deliverer.handleJob); err != nil {
		logger.Error("failed to register product events queue handler", log.Err(err))
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
		OnJobFailed: func(ctx context.Context, job queue.Job) {
			logger.ErrorContext(ctx, "product event delivery permanently failed",
				slog.Int64("job_id", job.ID),
				slog.Int("attempts", job.Attempts),
				slog.String("last_error", job.LastError.String),
			)
		},
	})
	if err != nil {
		logger.Error("failed to initialize product events worker", log.Err(err))
		return
	}

	var cancel context.CancelFunc
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			workerCtx, workerCancel := context.WithCancel(context.Background()) //nolint:gosec // canceled in OnStop
			cancel = workerCancel
			go func() {
				if runErr := worker.Run(workerCtx); runErr != nil {
					logger.ErrorContext(workerCtx, "product events worker stopped", log.Err(runErr))
				}
			}()
			logger.InfoContext(ctx, "product events worker started", slog.String("queue_name", queueName))
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
	logger    *slog.Logger
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

	eventType := payload.Type
	if eventType == "" {
		var env Envelope
		if unmarshalErr := json.Unmarshal(payload.Body, &env); unmarshalErr == nil {
			eventType = env.Type
		}
	}
	if !target.Allows(eventType) {
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
		d.recordDeliveryResult(ctx, payload.ProductID, target.URL, false)
		return fmt.Errorf("events: deliver: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDeliveryBodyBytes))

	if resp.StatusCode >= successStatusMin && resp.StatusCode < successStatusMax {
		d.recordDeliveryResult(ctx, payload.ProductID, target.URL, true)
		return nil
	}
	// Standard Webhooks: "If a delivery target has been retired, but the HTTP
	// site still exists, the site SHOULD return a 410 Gone status code and the
	// sender SHOULD refrain from sending any further notifications."
	if resp.StatusCode == goneStatus {
		if delErr := d.repo.DeleteByProductIDInternal(ctx, payload.ProductID); delErr != nil {
			return fmt.Errorf("events: disable gone endpoint: %w", delErr)
		}
		return nil
	}
	d.recordDeliveryResult(ctx, payload.ProductID, target.URL, false)
	d.logger.WarnContext(ctx, "product event delivery rejected",
		slog.Int("status", resp.StatusCode),
		slog.String("event_id", payload.EventID),
	)
	return fmt.Errorf("events: delivery status %d", resp.StatusCode)
}

func (d *deliverer) recordDeliveryResult(ctx context.Context, productID, endpointURL string, succeeded bool) {
	if err := d.repo.RecordDeliveryResultInternal(ctx, productID, endpointURL, succeeded); err != nil {
		d.logger.ErrorContext(ctx, "failed to record event delivery result",
			log.Err(err),
			slog.String("product_id", productID),
		)
	}
}

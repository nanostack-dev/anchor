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

	"anchor/internal/queueworker"

	"github.com/nanostack-dev/pgkit/queue"
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

// Worker delivers the product events to the products' endpoints.
type Worker struct {
	queueworker.Worker
}

func NewWorker(
	queueClient *queue.Client,
	endpointService EndpointService,
	endpoints EndpointRepository,
	logger *slog.Logger,
	core *serviceconfig.CoreConfig,
) (*Worker, error) {
	logger = logger.With(slog.String("component", "product_events_worker"))
	deliverer := &deliverer{
		endpoints: endpointService,
		repo:      endpoints,
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
		return nil, fmt.Errorf("register product events queue handler: %w", err)
	}

	pollInterval := workerPollInterval
	if core != nil && !core.IsProduction() {
		pollInterval = workerPollIntervalDev
	}

	worker, err := queue.NewWorker(queueClient, registry, queue.WorkerConfig{
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
		return nil, fmt.Errorf("initialize product events worker: %w", err)
	}

	return &Worker{queueworker.New(worker, logger, queueworker.Spec{
		Started:      "product events worker started",
		StartedAttrs: []slog.Attr{slog.String("queue_name", queueName)},
		Failed:       "product events worker stopped",
	})}, nil
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

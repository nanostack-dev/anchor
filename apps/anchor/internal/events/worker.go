package events

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	serviceconfig "anchor/internal/service/config"

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
	deliveryTimeout       = 15 * time.Second
	maxDeliveryBodyBytes  = 1 << 20
	successStatusMin      = 200
	successStatusMax      = 300
	goneStatus            = 410
	// goneMinFailedCalls is how many failed calls in a row, the last one a
	// 410, remove an endpoint, once the streak is also older than the
	// policy's goneMinFailingFor. One 410 can come from a proxy or a deploy in
	// progress, and removing the endpoint loses its signing secret.
	goneMinFailedCalls = 3
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
	retry := deliveryRetryPolicyFor(core)
	deliverer := &deliverer{
		endpoints: endpointService,
		repo:      endpoints,
		http:      newDeliveryClient(core != nil && core.Events.AllowPrivateEndpoints),
		policy:    retry,
		now:       time.Now,
		logger:    logger,
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
		BackoffBase:       retry.base,
		BackoffMax:        retry.max,
		RetryDelay:        retry.retryDelay,
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
	policy    deliveryRetryPolicy
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
	// The attempt is spent without a call and without counting as a failed
	// call: the job retries on its own schedule, and the endpoint's next call
	// after the cooldown decides whether it is back.
	attemptAt := d.now()
	if wait := d.policy.breakerWait(target, attemptAt); wait > 0 {
		return fmt.Errorf("%w: next call in %s", errEndpointCoolingDown, wait.Round(time.Second))
	}

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
		// A refused address does not become public by waiting: the job fails
		// now instead of spending its day of attempts.
		if errors.Is(err, errNonPublicAddress) {
			return queue.NonRetryable(fmt.Errorf("events: deliver: %w", err))
		}
		return fmt.Errorf("events: deliver: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDeliveryBodyBytes))

	if resp.StatusCode >= successStatusMin && resp.StatusCode < successStatusMax {
		d.recordDeliveryResult(ctx, payload.ProductID, target.URL, true)
		return nil
	}
	d.recordDeliveryResult(ctx, payload.ProductID, target.URL, false)
	// Standard Webhooks: "If a delivery target has been retired, but the HTTP
	// site still exists, the site SHOULD return a 410 Gone status code and the
	// sender SHOULD refrain from sending any further notifications." Only the
	// URL that answered is removed, and only after repeated failures over a
	// long streak.
	if resp.StatusCode == goneStatus {
		removed, delErr := d.repo.DeleteGoneEndpointInternal(
			ctx, payload.ProductID, target.URL, goneMinFailedCalls, d.policy.goneMinFailingFor,
		)
		if delErr != nil {
			return fmt.Errorf("events: remove gone endpoint: %w", delErr)
		}
		if removed {
			d.logger.WarnContext(ctx, "product event endpoint removed after repeated 410 Gone",
				slog.String("product_id", payload.ProductID),
				slog.String("event_id", payload.EventID),
			)
			return nil
		}
	}
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

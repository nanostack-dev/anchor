package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/nanostack-dev/pgkit/queue"

	serviceconfig "anchor/internal/service/config"
)

func ValidateEndpointURLForTest(raw string, production bool) error {
	return validateEndpointURL(raw, production)
}

func DeliveredDataForTest(data Data) Data {
	return deliveredData(data)
}

// DeliveryRetryWindowForTest reports the attempts a delivery gets and the
// shortest and longest time from its first attempt to its last.
func DeliveryRetryWindowForTest(core *serviceconfig.CoreConfig) (int, time.Duration, time.Duration) {
	policy := deliveryRetryPolicyFor(core)
	shortest, longest := policy.retryWindow()
	return policy.maxAttempts, shortest, longest
}

// DeliveryRetryDelayForTest is the worker's delay after the given attempt of a job.
func DeliveryRetryDelayForTest(core *serviceconfig.CoreConfig, jobID int64, attempt int) time.Duration {
	return deliveryRetryPolicyFor(core).retryDelay(queue.Job{ID: jobID, Attempts: attempt}, nil)
}

// DeliverForTest runs one attempt of the delivery job for the event, with the
// given endpoint lookup, repository and HTTP client.
func DeliverForTest(
	ctx context.Context, endpoints EndpointService, repo EndpointRepository, client *http.Client, productID string,
) error {
	d := &deliverer{
		endpoints: endpoints,
		repo:      repo,
		http:      client,
		now:       time.Now,
		logger:    slog.New(slog.DiscardHandler),
	}
	payload, err := json.Marshal(queuePayload{
		EventID:   "pevt_test",
		ProductID: productID,
		Type:      OrganizationCreated,
		Body:      json.RawMessage(`{"type":"organization.created","timestamp":"2026-11-19T00:00:00Z","data":{}}`),
	})
	if err != nil {
		return err
	}
	return d.handleJob(ctx, queue.Job{ID: 1, Attempts: 1, Payload: payload})
}

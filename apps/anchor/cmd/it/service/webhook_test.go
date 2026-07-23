package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/license"
	"anchor/internal/domain/organization"
	"anchor/internal/domain/plan"
	"anchor/internal/domain/webhook"
	"anchor/internal/repository"
)

const (
	webhookFanoutQueueName  = "webhook.fanout"
	webhookDeliverQueueName = "webhook.deliver"
	webhookJobListLimit     = 1000
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

type webhookContext struct {
	ProductID      string
	OrganizationID string
	Plan           plan.Plan
}

func givenWebhookContext(t *testing.T) webhookContext {
	t.Helper()

	tenantAndProduct := GivenATenantAndProduct(t)
	productID := tenantAndProduct.Product.ID

	org := organization.Organization{
		ProductID: productID,
		Name:      Faker.RandomStringWithLength(20),
	}
	org.GenerateID()
	createdOrg, err := OrganizationRepo.Create(t.Context(), org)
	require.NoError(t, err)

	createdPlan, err := PlanService.Create(t.Context(), plan.CreatePlanInput{
		ProductID: productID,
		Key:       "pro",
		Name:      "Pro",
	})
	require.NoError(t, err)

	return webhookContext{
		ProductID:      productID,
		OrganizationID: createdOrg.ID,
		Plan:           createdPlan,
	}
}

func givenWebhookEndpoint(
	t *testing.T, productID string, targetURL string, eventTypes []string,
) webhook.Endpoint {
	t.Helper()

	created, err := WebhookEndpointSvc.Create(t.Context(), webhook.CreateEndpointInput{
		ProductID:   productID,
		URL:         targetURL,
		Description: "integration test endpoint",
		EventTypes:  eventTypes,
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.PlaintextSecret)

	return created.Endpoint
}

func givenLicense(t *testing.T, ctxData webhookContext) license.License {
	t.Helper()

	created, err := LicenseService.Put(t.Context(), license.PutLicenseInput{
		ProductID:      ctxData.ProductID,
		OrganizationID: ctxData.OrganizationID,
		PlanID:         ctxData.Plan.ID,
	})
	require.NoError(t, err)

	return created
}

// findFanoutJob locates the pending fan-out job whose event has the given type
// and organization. Asserting on the job proves the outbox row and the job were
// written by the business transaction, not by a background poller.
func findFanoutJob(
	t *testing.T, eventType string, organizationID string,
) (*pgqueue.Job, *webhook.Event) {
	t.Helper()

	jobs, err := Queue.ListJobs(context.Background(), pgqueue.ListJobsParams{
		QueueName: webhookFanoutQueueName,
		Status:    pgqueue.StatusPending,
		Limit:     webhookJobListLimit,
	})
	require.NoError(t, err)

	for _, job := range jobs {
		var payload struct {
			EventID string `json:"event_id"`
		}
		if jsonErr := json.Unmarshal(job.Payload, &payload); jsonErr != nil {
			continue
		}
		event, findErr := WebhookEventRepo.FindByIDInternal(t.Context(), payload.EventID)
		require.NoError(t, findErr)
		if event == nil || event.EventType != eventType {
			continue
		}
		if organizationID != "" &&
			(event.OrganizationID == nil || *event.OrganizationID != organizationID) {
			continue
		}

		jobCopy := job
		return &jobCopy, event
	}

	return nil, nil
}

func countDeliverJobs(t *testing.T, deliveryID string) int {
	t.Helper()

	jobs, err := Queue.ListJobs(context.Background(), pgqueue.ListJobsParams{
		QueueName: webhookDeliverQueueName,
		Status:    pgqueue.StatusPending,
		Limit:     webhookJobListLimit,
	})
	require.NoError(t, err)

	count := 0
	for _, job := range jobs {
		var payload struct {
			DeliveryID string `json:"delivery_id"`
		}
		if jsonErr := json.Unmarshal(job.Payload, &payload); jsonErr != nil {
			continue
		}
		if payload.DeliveryID == deliveryID {
			count++
		}
	}

	return count
}

func reloadDelivery(t *testing.T, deliveryID string) webhook.Delivery {
	t.Helper()

	found, err := WebhookDeliveryRepo.FindByIDInternal(t.Context(), deliveryID)
	require.NoError(t, err)
	require.NotNil(t, found)

	return *found
}

func reloadEndpoint(t *testing.T, endpointID string) webhook.Endpoint {
	t.Helper()

	found, err := WebhookEndpointRepo.FindByIDInternal(t.Context(), endpointID)
	require.NoError(t, err)
	require.NotNil(t, found)

	return *found
}

// ---------------------------------------------------------------------------
// Emission
// ---------------------------------------------------------------------------

func TestLicenseWritesEmitWebhookEvents(t *testing.T) {
	t.Run("assigning a license emits license.created with a fanout job", func(t *testing.T) {
		ctxData := givenWebhookContext(t)
		created := givenLicense(t, ctxData)

		job, event := findFanoutJob(
			t, webhook.EventTypeLicenseCreated, ctxData.OrganizationID,
		)
		require.NotNil(t, job, "the business transaction must have enqueued a fanout job")
		require.NotNil(t, event)

		assert.Equal(t, ctxData.ProductID, event.ProductID)
		assert.Equal(t, webhook.APIVersion, event.APIVersion)

		var data webhook.LicenseEventData
		require.NoError(t, json.Unmarshal(event.Payload, &data))
		assert.Equal(t, created.ID, data.LicenseID)
		assert.Equal(t, "pro", data.PlanKey)
		assert.Equal(t, string(license.StatusActive), data.Status)
	})

	t.Run("suspending emits license.updated carrying the status transition", func(t *testing.T) {
		ctxData := givenWebhookContext(t)
		givenLicense(t, ctxData)

		_, err := LicenseService.Suspend(t.Context(), license.SuspendLicenseInput{
			ProductID:      ctxData.ProductID,
			OrganizationID: ctxData.OrganizationID,
		})
		require.NoError(t, err)

		_, event := findFanoutJob(t, webhook.EventTypeLicenseUpdated, ctxData.OrganizationID)
		require.NotNil(t, event)

		var data webhook.LicenseEventData
		require.NoError(t, json.Unmarshal(event.Payload, &data))
		assert.Equal(t, string(license.StatusSuspended), data.Status)
		require.Contains(t, data.Changes, "status")
		assert.Equal(t, string(license.StatusActive), data.Changes["status"].Previous)
		assert.Equal(t, string(license.StatusSuspended), data.Changes["status"].New)
	})

	t.Run("revoking emits license.revoked, not license.updated", func(t *testing.T) {
		ctxData := givenWebhookContext(t)
		givenLicense(t, ctxData)

		_, err := LicenseService.Revoke(t.Context(), license.RevokeLicenseInput{
			ProductID:      ctxData.ProductID,
			OrganizationID: ctxData.OrganizationID,
		})
		require.NoError(t, err)

		_, revoked := findFanoutJob(t, webhook.EventTypeLicenseRevoked, ctxData.OrganizationID)
		require.NotNil(t, revoked, "revocation gets its own event type")

		_, updated := findFanoutJob(t, webhook.EventTypeLicenseUpdated, ctxData.OrganizationID)
		assert.Nil(t, updated, "a revoke must not also emit license.updated")
	})

	t.Run("updating a plan emits once at product scope", func(t *testing.T) {
		ctxData := givenWebhookContext(t)
		// A licensed organization exists: a per-organization emit would produce
		// one event per licensee for a single plan edit.
		givenLicense(t, ctxData)

		_, err := PlanService.Update(t.Context(), plan.UpdatePlanInput{
			ProductID: ctxData.ProductID,
			PlanID:    ctxData.Plan.ID,
			Name:      new("Pro v2"),
		})
		require.NoError(t, err)

		job, event := findFanoutJob(t, webhook.EventTypePlanUpdated, "")
		require.NotNil(t, job)
		require.NotNil(t, event)
		assert.Nil(t, event.OrganizationID, "plan.updated is product-scoped")

		var data webhook.PlanEventData
		require.NoError(t, json.Unmarshal(event.Payload, &data))
		assert.Equal(t, ctxData.Plan.ID, data.PlanID)
		assert.Equal(t, "pro", data.PlanKey)
	})
}

// ---------------------------------------------------------------------------
// Fan-out
// ---------------------------------------------------------------------------

func TestWebhookFanout(t *testing.T) {
	t.Run("creates deliveries only for subscribed enabled endpoints", func(t *testing.T) {
		ctxData := givenWebhookContext(t)

		subscribed := givenWebhookEndpoint(
			t, ctxData.ProductID, "https://receiver.example.com/a", []string{"license.*"},
		)
		otherGroup := givenWebhookEndpoint(
			t, ctxData.ProductID, "https://receiver.example.com/b", []string{"plan.updated"},
		)
		disabled := givenWebhookEndpoint(
			t, ctxData.ProductID, "https://receiver.example.com/c", []string{"license.*"},
		)
		_, err := WebhookEndpointSvc.SetEnabled(t.Context(), webhook.SetEndpointEnabledInput{
			ProductID:  ctxData.ProductID,
			EndpointID: disabled.ID,
			Enabled:    false,
		})
		require.NoError(t, err)

		givenLicense(t, ctxData)
		job, event := findFanoutJob(
			t, webhook.EventTypeLicenseCreated, ctxData.OrganizationID,
		)
		require.NotNil(t, job)

		// Invoke the handler directly rather than racing the poller.
		require.NoError(t, WebhookFanoutSvc.ProcessQueueJob(context.Background(), *job))

		deliveries := listDeliveries(t, ctxData.ProductID, subscribed.ID)
		require.Len(t, deliveries, 1)
		assert.Equal(t, event.ID, deliveries[0].Delivery.EventID)
		assert.Equal(t, webhook.DeliveryStatusPending, deliveries[0].Delivery.Status)
		assert.Equal(
			t, 1, countDeliverJobs(t, deliveries[0].Delivery.ID),
			"each delivery gets exactly one deliver job",
		)

		assert.Empty(
			t, listDeliveries(t, ctxData.ProductID, otherGroup.ID),
			"an endpoint subscribed to another group must not receive the event",
		)
		assert.Empty(
			t, listDeliveries(t, ctxData.ProductID, disabled.ID),
			"a disabled endpoint must not accrue deliveries",
		)
	})

	t.Run("re-running fanout does not double-deliver", func(t *testing.T) {
		ctxData := givenWebhookContext(t)
		endpoint := givenWebhookEndpoint(
			t, ctxData.ProductID, "https://receiver.example.com/idempotent",
			[]string{webhook.EventTypeLicenseCreated},
		)

		givenLicense(t, ctxData)
		job, _ := findFanoutJob(t, webhook.EventTypeLicenseCreated, ctxData.OrganizationID)
		require.NotNil(t, job)

		require.NoError(t, WebhookFanoutSvc.ProcessQueueJob(context.Background(), *job))
		require.NoError(t, WebhookFanoutSvc.ProcessQueueJob(context.Background(), *job))

		deliveries := listDeliveries(t, ctxData.ProductID, endpoint.ID)
		assert.Len(t, deliveries, 1, "the crash-safe re-run must find the existing row")
	})

	t.Run("a ping reaches only its target endpoint", func(t *testing.T) {
		ctxData := givenWebhookContext(t)
		target := givenWebhookEndpoint(
			t, ctxData.ProductID, "https://receiver.example.com/target",
			[]string{"license.*"},
		)
		bystander := givenWebhookEndpoint(
			t, ctxData.ProductID, "https://receiver.example.com/bystander",
			[]string{"license.*", webhook.EventTypePing},
		)

		event, err := WebhookEndpointSvc.Ping(t.Context(), webhook.PingEndpointInput{
			ProductID:  ctxData.ProductID,
			EndpointID: target.ID,
		})
		require.NoError(t, err)
		require.NotEmpty(t, event.ID)

		job, _ := findFanoutJob(t, webhook.EventTypePing, "")
		require.NotNil(t, job)
		require.NoError(t, WebhookFanoutSvc.ProcessQueueJob(context.Background(), *job))

		assert.Len(t, listDeliveries(t, ctxData.ProductID, target.ID), 1)
		assert.Empty(
			t, listDeliveries(t, ctxData.ProductID, bystander.ID),
			"a ping is aimed at one endpoint even when others subscribe to ping",
		)
	})
}

func listDeliveries(
	t *testing.T, productID string, endpointID string,
) []webhook.DeliveryWithEvent {
	t.Helper()

	deliveries, err := WebhookEndpointSvc.ListDeliveries(
		t.Context(), webhook.ListDeliveriesInput{
			ProductID:  productID,
			EndpointID: endpointID,
		},
	)
	require.NoError(t, err)

	return deliveries
}

// ---------------------------------------------------------------------------
// Delivery
// ---------------------------------------------------------------------------

// fanOutOneDelivery drives a license event through fan-out and returns the
// single delivery it produced for the endpoint.
func fanOutOneDelivery(
	t *testing.T, ctxData webhookContext, endpoint webhook.Endpoint,
) webhook.Delivery {
	t.Helper()

	givenLicense(t, ctxData)
	job, _ := findFanoutJob(t, webhook.EventTypeLicenseCreated, ctxData.OrganizationID)
	require.NotNil(t, job)
	require.NoError(t, WebhookFanoutSvc.ProcessQueueJob(context.Background(), *job))

	deliveries := listDeliveries(t, ctxData.ProductID, endpoint.ID)
	require.Len(t, deliveries, 1)

	return deliveries[0].Delivery
}

func TestWebhookDeliverySuccessPath(t *testing.T) {
	receiver := startWebhookReceiver(t)
	receiver.RespondWith(t, http.StatusOK, `{"ok":true}`)

	ctxData := givenWebhookContext(t)
	endpoint := givenWebhookEndpoint(
		t, ctxData.ProductID, receiver.TargetURL(), []string{"license.*"},
	)
	delivery := fanOutOneDelivery(t, ctxData, endpoint)

	require.NoError(t, WebhookDeliverySvc.DeliverOnce(context.Background(), delivery.ID))

	stored := reloadDelivery(t, delivery.ID)
	assert.Equal(t, webhook.DeliveryStatusSucceeded, stored.Status)
	assert.Equal(t, int32(1), stored.AttemptCount)
	require.NotNil(t, stored.LastStatusCode)
	assert.Equal(t, int32(http.StatusOK), *stored.LastStatusCode)
	require.NotNil(t, stored.CompletedAt)

	attempts, err := WebhookDeliveryRepo.ListAttempts(t.Context(), delivery.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	assert.Equal(t, int32(1), attempts[0].AttemptNumber)
	require.NotNil(t, attempts[0].StatusCode)
	assert.Equal(t, int32(http.StatusOK), *attempts[0].StatusCode)
	require.NotNil(t, attempts[0].ResponseSnippet)
	assert.JSONEq(t, `{"ok":true}`, *attempts[0].ResponseSnippet)

	// The endpoint's failure streak is cleared by a success.
	reloaded := reloadEndpoint(t, endpoint.ID)
	assert.Equal(t, int32(0), reloaded.ConsecutiveFailureCount)
	assert.NotNil(t, reloaded.LastSuccessAt)

	// The signature headers must have arrived exactly as the spec defines them.
	requests := receiver.RecordedRequests(t)
	require.Len(t, requests, 1)
	received := requests[0]

	assert.Equal(t, delivery.ID, received.header(webhook.HeaderWebhookID))
	assert.Equal(t, delivery.SignedBody, received.Body, "the frozen body is sent verbatim")

	timestamp := received.header(webhook.HeaderWebhookTimestamp)
	require.NotEmpty(t, timestamp)
	assert.WithinDuration(t, time.Now(), parseUnixHeader(t, timestamp), time.Minute)

	signature := received.header(webhook.HeaderWebhookSignature)
	require.NotEmpty(t, signature)
	entries := strings.Split(signature, " ")
	require.Len(t, entries, 1, "one entry per usable secret")
	assert.True(t, strings.HasPrefix(entries[0], webhook.SignatureVersion+","))
}

func parseUnixHeader(t *testing.T, header string) time.Time {
	t.Helper()

	seconds, err := strconv.ParseInt(header, 10, 64)
	require.NoError(t, err)

	return time.Unix(seconds, 0)
}

func TestWebhookDeliveryFailurePaths(t *testing.T) {
	t.Run("a 500 keeps the delivery pending and asks the queue to retry", func(t *testing.T) {
		receiver := startWebhookReceiver(t)
		receiver.RespondWith(t, http.StatusInternalServerError, "boom")

		ctxData := givenWebhookContext(t)
		endpoint := givenWebhookEndpoint(
			t, ctxData.ProductID, receiver.TargetURL(), []string{"license.*"},
		)
		delivery := fanOutOneDelivery(t, ctxData, endpoint)

		err := WebhookDeliverySvc.DeliverOnce(context.Background(), delivery.ID)
		require.Error(t, err)
		assert.False(
			t, pgqueue.IsNonRetryable(err),
			"a 5xx is transient, so the job must stay retryable",
		)

		stored := reloadDelivery(t, delivery.ID)
		assert.Equal(t, webhook.DeliveryStatusPending, stored.Status)
		assert.Equal(t, int32(1), stored.AttemptCount)
		assert.Nil(t, stored.CompletedAt)

		reloaded := reloadEndpoint(t, endpoint.ID)
		assert.Equal(t, int32(1), reloaded.ConsecutiveFailureCount)
		assert.NotNil(t, reloaded.FirstFailureAt)
		assert.Equal(t, webhook.EndpointStatusEnabled, reloaded.Status)
	})

	t.Run("a 404 fails permanently instead of burning the ladder", func(t *testing.T) {
		receiver := startWebhookReceiver(t)
		receiver.RespondWith(t, http.StatusNotFound, "no such hook")

		ctxData := givenWebhookContext(t)
		endpoint := givenWebhookEndpoint(
			t, ctxData.ProductID, receiver.TargetURL(), []string{"license.*"},
		)
		delivery := fanOutOneDelivery(t, ctxData, endpoint)

		err := WebhookDeliverySvc.DeliverOnce(context.Background(), delivery.ID)
		require.Error(t, err)
		assert.True(t, pgqueue.IsNonRetryable(err))

		stored := reloadDelivery(t, delivery.ID)
		assert.Equal(t, webhook.DeliveryStatusFailed, stored.Status)
		assert.Equal(t, int32(1), stored.AttemptCount)
	})

	t.Run("the last rung marks the delivery EXHAUSTED", func(t *testing.T) {
		receiver := startWebhookReceiver(t)
		receiver.RespondWith(t, http.StatusServiceUnavailable, "down")

		ctxData := givenWebhookContext(t)
		endpoint := givenWebhookEndpoint(
			t, ctxData.ProductID, receiver.TargetURL(), []string{"license.*"},
		)
		delivery := fanOutOneDelivery(t, ctxData, endpoint)

		// Fast-forward to the final rung instead of making eight real requests.
		require.NoError(t, WebhookDeliveryRepo.UpdateOutcomeInternal(
			t.Context(), delivery.ID, repository.DeliveryOutcome{
				Status:       webhook.DeliveryStatusPending,
				AttemptCount: webhook.MaxDeliveryAttempts - 1,
			},
		))

		err := WebhookDeliverySvc.DeliverOnce(context.Background(), delivery.ID)
		require.Error(t, err)
		assert.True(t, pgqueue.IsNonRetryable(err), "an exhausted delivery must not be requeued")

		stored := reloadDelivery(t, delivery.ID)
		assert.Equal(t, webhook.DeliveryStatusExhausted, stored.Status)
		assert.Equal(t, webhook.MaxDeliveryAttempts, stored.AttemptCount)
		require.NotNil(t, stored.CompletedAt)
	})

	t.Run("410 Gone disables the endpoint outright", func(t *testing.T) {
		receiver := startWebhookReceiver(t)
		receiver.RespondWith(t, http.StatusGone, "gone")

		ctxData := givenWebhookContext(t)
		endpoint := givenWebhookEndpoint(
			t, ctxData.ProductID, receiver.TargetURL(), []string{"license.*"},
		)
		delivery := fanOutOneDelivery(t, ctxData, endpoint)

		err := WebhookDeliverySvc.DeliverOnce(context.Background(), delivery.ID)
		require.Error(t, err)
		assert.True(t, pgqueue.IsNonRetryable(err))

		reloaded := reloadEndpoint(t, endpoint.ID)
		assert.Equal(t, webhook.EndpointStatusAutoDisabled, reloaded.Status)
		assert.Equal(t, webhook.GoneDisableReason, reloaded.DisabledReason)
	})
}

func TestWebhookAutoDisableRequiresBothConditions(t *testing.T) {
	t.Run("a young failure streak does not disable the endpoint", func(t *testing.T) {
		receiver := startWebhookReceiver(t)
		receiver.RespondWith(t, http.StatusInternalServerError, "boom")

		ctxData := givenWebhookContext(t)
		endpoint := givenWebhookEndpoint(
			t, ctxData.ProductID, receiver.TargetURL(), []string{"license.*"},
		)
		delivery := fanOutOneDelivery(t, ctxData, endpoint)

		// Well past the failure threshold, but the streak started minutes ago:
		// a deploy blip must not cost a customer their integration.
		recent := time.Now().Add(-10 * time.Minute)
		seedFailureStreak(t, endpoint.ID, webhook.AutoDisableFailureThreshold+5, recent)

		_ = WebhookDeliverySvc.DeliverOnce(context.Background(), delivery.ID)

		reloaded := reloadEndpoint(t, endpoint.ID)
		assert.Equal(t, webhook.EndpointStatusEnabled, reloaded.Status)
		assert.Empty(t, reloaded.DisabledReason)
	})

	t.Run("a long-running streak past the threshold disables the endpoint", func(t *testing.T) {
		receiver := startWebhookReceiver(t)
		receiver.RespondWith(t, http.StatusInternalServerError, "boom")

		ctxData := givenWebhookContext(t)
		endpoint := givenWebhookEndpoint(
			t, ctxData.ProductID, receiver.TargetURL(), []string{"license.*"},
		)
		delivery := fanOutOneDelivery(t, ctxData, endpoint)

		// One short of the threshold, started more than a day ago: this attempt
		// is the one that satisfies both conditions at once.
		old := time.Now().Add(-30 * time.Hour)
		seedFailureStreak(t, endpoint.ID, webhook.AutoDisableFailureThreshold-1, old)

		_ = WebhookDeliverySvc.DeliverOnce(context.Background(), delivery.ID)

		reloaded := reloadEndpoint(t, endpoint.ID)
		assert.Equal(t, webhook.EndpointStatusAutoDisabled, reloaded.Status)
		assert.Equal(t, webhook.AutoDisableReason, reloaded.DisabledReason)
		assert.Equal(t, webhook.AutoDisableFailureThreshold, reloaded.ConsecutiveFailureCount)
	})
}

func seedFailureStreak(
	t *testing.T, endpointID string, failures int32, firstFailureAt time.Time,
) {
	t.Helper()

	require.NoError(t, WebhookEndpointRepo.UpdateHealthInternal(
		t.Context(),
		endpointID,
		webhook.FailureCounters{
			ConsecutiveFailureCount: failures,
			FirstFailureAt:          &firstFailureAt,
			LastFailureAt:           &firstFailureAt,
		},
		webhook.EndpointStatusEnabled,
		"",
	))
}

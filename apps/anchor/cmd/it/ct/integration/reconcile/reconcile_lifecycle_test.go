package reconcile_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schedulerEventualTimeout / schedulerPollInterval define timing for require.Eventually
// assertions. The scheduler job is enqueued synchronously after the HTTP response
// commits, so 3 s is a generous upper bound.
const (
	schedulerEventualTimeout = 3 * time.Second
	schedulerPollInterval    = 100 * time.Millisecond
)

// fakeAPIKey is a syntactically valid but non-functional Clerk API key used to
// exercise the scheduler lifecycle without hitting real Clerk services.
const fakeAPIKey = "sk_test_scheduler_lifecycle_fake_key_for_testing"

func TestSchedulerLifecycle_NoKeyOnCreate_NoSchedulerJob(t *testing.T) {
	before := countPendingSchedulerJobs(t)

	productContext := createTestProductContext(t)

	// Create a Clerk instance WITHOUT an API key.
	createBody := ct.CreateIntegrationInstanceJSONRequestBody{}
	require.NoError(
		t,
		createBody.FromClerkIntegrationInstanceCreateRequest(
			ct.ClerkIntegrationInstanceCreateRequest{ProviderType: "CLERK"},
		),
	)

	resp, err := productContext.OwnerAuthenticatedClient().CreateIntegrationInstanceWithResponse(
		context.Background(),
		productContext.ProductID,
		createBody,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode())

	// Give the server a moment to process any side effects, then assert no
	// new scheduler job was added.
	time.Sleep(200 * time.Millisecond)

	after := countPendingSchedulerJobs(t)
	assert.Equal(t, before, after, "no scheduler job should be added when no API key is configured")
}

func TestSchedulerLifecycle_FirstKeyAdded_SeedsScheduler(t *testing.T) {
	// Each sub-test that mutates the queue must run serially to avoid
	// interference with the RemoveLastKey / DeleteLastInstance tests that
	// assert queue drains to zero.  These additive tests are safe to run in
	// any order; the cancel tests are collected in their own group below.

	productContext := createTestProductContext(t)
	instance := createInstanceWithAPIKey(t, productContext, fakeAPIKey)
	t.Cleanup(func() { deleteInstance(t, productContext, instance.Id) })

	// A scheduler job must appear shortly after the API call returns.
	require.Eventually(t, func() bool {
		return countPendingSchedulerJobs(t) >= 1
	}, schedulerEventualTimeout, schedulerPollInterval,
		"a scheduler job should be seeded when the first Clerk API key is added")
}

func TestSchedulerLifecycle_SecondKeyAdded_DoesNotDuplicate(t *testing.T) {
	// Seed the queue with the first instance.
	productContextA := createTestProductContext(t)
	instanceA := createInstanceWithAPIKey(t, productContextA, fakeAPIKey)
	t.Cleanup(func() { deleteInstance(t, productContextA, instanceA.Id) })

	require.Eventually(t, func() bool {
		return countPendingSchedulerJobs(t) >= 1
	}, schedulerEventualTimeout, schedulerPollInterval, "first scheduler job should appear")

	countAfterFirst := countPendingSchedulerJobs(t)

	// Add a second instance with an API key on a different product.
	productContextB := createTestProductContext(t)
	instanceB := createInstanceWithAPIKey(t, productContextB, fakeAPIKey)
	t.Cleanup(func() { deleteInstance(t, productContextB, instanceB.Id) })

	// Wait briefly, then assert the scheduler count did not grow.
	time.Sleep(300 * time.Millisecond)
	countAfterSecond := countPendingSchedulerJobs(t)

	assert.Equal(t, countAfterFirst, countAfterSecond,
		"adding a second Clerk API key should not seed a duplicate scheduler job")
}

func TestSchedulerLifecycle_RemoveLastKey_CancelsScheduler(t *testing.T) {
	// This test must run after all additive tests have already seeded at
	// least one scheduler job (or with a clean queue) and must be the sole
	// consumer of that state. Because test isolation within a shared server
	// is tricky we accept that the assertion is "eventually zero" — meaning
	// all keys across all products have been removed.
	//
	// Strategy: we create the instance, confirm the scheduler job appears, then
	// remove the key. Because removeAPIKey patches *only* the config field
	// and passes api_key: nil, the service will call maybeCancelScheduler which
	// deletes pending scheduler jobs when no instance with a key remains.

	productContext := createTestProductContext(t)
	instance := createInstanceWithAPIKey(t, productContext, fakeAPIKey)

	require.Eventually(t, func() bool {
		return countPendingSchedulerJobs(t) >= 1
	}, schedulerEventualTimeout, schedulerPollInterval, "scheduler job should appear after key added")

	removeAPIKey(t, productContext, instance.Id)

	require.Eventually(t, func() bool {
		return countPendingSchedulerJobs(t) == 0
	}, schedulerEventualTimeout, schedulerPollInterval,
		"scheduler job should be cancelled after the last API key is removed")
}

func TestSchedulerLifecycle_DeleteLastInstance_CancelsScheduler(t *testing.T) {
	productContext := createTestProductContext(t)
	instance := createInstanceWithAPIKey(t, productContext, fakeAPIKey)

	require.Eventually(t, func() bool {
		return countPendingSchedulerJobs(t) >= 1
	}, schedulerEventualTimeout, schedulerPollInterval, "scheduler job should appear after key added")

	deleteInstance(t, productContext, instance.Id)

	require.Eventually(t, func() bool {
		return countPendingSchedulerJobs(t) == 0
	}, schedulerEventualTimeout, schedulerPollInterval,
		"scheduler job should be cancelled after the instance with the last API key is deleted")
}

package events_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"anchor/internal/events"

	"github.com/nanostack-dev/pgkit/queue"
	"github.com/stretchr/testify/require"
)

// fakeEndpoints answers the worker's endpoint lookup with a fixed target and,
// when store is set, the failure streak that store keeps for it.
type fakeEndpoints struct {
	events.EndpointService

	target events.DeliveryTarget
	store  *fakeEndpointStore
}

func (f fakeEndpoints) DeliveryTarget(context.Context, string) (events.DeliveryTarget, bool, error) {
	target := f.target
	if f.store != nil {
		f.store.mu.Lock()
		target.ConsecutiveFailedCalls = f.store.failures
		target.LastFailedAt = f.store.lastFailedAt
		f.store.mu.Unlock()
	}
	return target, true, nil
}

// fakeEndpointStore keeps one stored endpoint in memory, with the delivery
// counters the repository keeps, dated by clock (nil is time.Now).
type fakeEndpointStore struct {
	events.EndpointRepository

	mu           sync.Mutex
	clock        *fakeClock
	url          string
	failures     int32
	failingSince time.Time
	lastFailedAt time.Time
	deleted      bool
}

func (f *fakeEndpointStore) now() time.Time {
	if f.clock == nil {
		return time.Now()
	}
	return f.clock.Now()
}

func (f *fakeEndpointStore) RecordDeliveryResultInternal(_ context.Context, _, url string, succeeded bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleted || url != f.url {
		return nil
	}
	if succeeded {
		f.failures = 0
		f.failingSince = time.Time{}
		f.lastFailedAt = time.Time{}
	} else {
		f.failures++
		if f.failingSince.IsZero() {
			f.failingSince = f.now()
		}
		f.lastFailedAt = f.now()
	}
	return nil
}

func (f *fakeEndpointStore) DeleteGoneEndpointInternal(
	_ context.Context,
	_, url string,
	minFailed int32,
	minFailingFor time.Duration,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleted || url != f.url || f.failures < minFailed || f.failingSince.IsZero() ||
		f.now().Sub(f.failingSince) < minFailingFor {
		return false, nil
	}
	f.deleted = true
	return true, nil
}

// fakeClock is a clock that a test moves by hand.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (f *fakeEndpointStore) isDeleted() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deleted
}

func countingServer(t *testing.T, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server, &hits
}

func goneServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	return countingServer(t, http.StatusGone)
}

func goneTarget(url string) events.DeliveryTarget {
	return events.DeliveryTarget{
		URL:    url,
		Secret: "whsec_dGVzdHNlY3JldHRlc3RzZWNyZXQ=",
		Events: []string{string(events.OrganizationCreated)},
	}
}

func TestDeliveryToAGoneEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("A single 410 keeps the endpoint and retries the delivery", func(t *testing.T) {
		t.Parallel()
		server, hits := goneServer(t)
		store := &fakeEndpointStore{url: server.URL}

		err := events.DeliverForTest(context.Background(), fakeEndpoints{target: goneTarget(server.URL)}, store,
			server.Client(), "prd_test")

		require.Error(t, err)
		require.False(t, store.isDeleted())
		require.EqualValues(t, 1, hits.Load())
	})

	t.Run("A 410 from an endpoint's old URL never removes its current URL", func(t *testing.T) {
		t.Parallel()
		server, _ := goneServer(t)
		store := &fakeEndpointStore{url: "https://example.com/new-hooks", failures: 5}

		for range 3 {
			err := events.DeliverForTest(context.Background(), fakeEndpoints{target: goneTarget(server.URL)}, store,
				server.Client(), "prd_test")
			require.Error(t, err)
		}

		require.False(t, store.isDeleted())
	})

	t.Run("Three failed calls in a row ending in 410 remove the endpoint and stop the delivery", func(t *testing.T) {
		t.Parallel()
		server, hits := goneServer(t)
		store := &fakeEndpointStore{url: server.URL}
		endpoints := fakeEndpoints{target: goneTarget(server.URL)}

		require.Error(t, events.DeliverForTest(context.Background(), endpoints, store, server.Client(), "prd_test"))
		require.Error(t, events.DeliverForTest(context.Background(), endpoints, store, server.Client(), "prd_test"))
		require.NoError(t, events.DeliverForTest(context.Background(), endpoints, store, server.Client(), "prd_test"))

		require.True(t, store.isDeleted())
		require.EqualValues(t, 3, hits.Load())
	})
}

func TestDeliveryToAGoneEndpointInADeployment(t *testing.T) {
	t.Parallel()

	t.Run("410s for half a minute keep the endpoint", func(t *testing.T) {
		t.Parallel()
		server, hits := goneServer(t)
		clock := newFakeClock()
		store := &fakeEndpointStore{clock: clock, url: server.URL}
		endpoints := fakeEndpoints{target: goneTarget(server.URL)}
		rules := events.DeployedDeliveryRulesForTest(clock.Now)

		for range 4 {
			require.Error(t, events.DeliverWithRulesForTest(t.Context(), endpoints, store, server.Client(), "prd_test",
				rules))
			clock.Advance(10 * time.Second)
		}

		require.False(t, store.isDeleted())
		require.EqualValues(t, 4, hits.Load())
	})

	t.Run("Two 503s and then one 410 keep the endpoint", func(t *testing.T) {
		t.Parallel()
		server, _ := goneServer(t)
		clock := newFakeClock()
		store := &fakeEndpointStore{
			clock: clock, url: server.URL, failures: 2,
			failingSince: clock.Now(), lastFailedAt: clock.Now(),
		}
		clock.Advance(time.Minute)

		err := events.DeliverWithRulesForTest(t.Context(), fakeEndpoints{target: goneTarget(server.URL)}, store,
			server.Client(), "prd_test", events.DeployedDeliveryRulesForTest(clock.Now))

		require.Error(t, err)
		require.False(t, store.isDeleted())
	})

	t.Run("A 410 after half a day of failed calls removes the endpoint", func(t *testing.T) {
		t.Parallel()
		server, _ := goneServer(t)
		clock := newFakeClock()
		store := &fakeEndpointStore{
			clock: clock, url: server.URL, failures: 2,
			failingSince: clock.Now(), lastFailedAt: clock.Now(),
		}
		clock.Advance(12 * time.Hour)

		err := events.DeliverWithRulesForTest(t.Context(), fakeEndpoints{target: goneTarget(server.URL)}, store,
			server.Client(), "prd_test", events.DeployedDeliveryRulesForTest(clock.Now))

		require.NoError(t, err)
		require.True(t, store.isDeleted())
	})
}

func TestDeliveryToAnEndpointThatKeepsFailing(t *testing.T) {
	t.Parallel()

	t.Run("Four failed calls in a row do not stop the next call", func(t *testing.T) {
		t.Parallel()
		server, hits := countingServer(t, http.StatusServiceUnavailable)
		clock := newFakeClock()
		store := &fakeEndpointStore{
			clock: clock, url: server.URL, failures: 4,
			failingSince: clock.Now(), lastFailedAt: clock.Now(),
		}

		err := events.DeliverWithRulesForTest(t.Context(), fakeEndpoints{target: goneTarget(server.URL), store: store},
			store, server.Client(), "prd_test", events.DeployedDeliveryRulesForTest(clock.Now))

		require.Error(t, err)
		require.NotErrorIs(t, err, events.ErrEndpointCoolingDownForTest)
		require.EqualValues(t, 1, hits.Load())
	})

	t.Run("After five failed calls in a row the endpoint is not called during its cooldown", func(t *testing.T) {
		t.Parallel()
		server, hits := countingServer(t, http.StatusServiceUnavailable)
		clock := newFakeClock()
		store := &fakeEndpointStore{
			clock: clock, url: server.URL, failures: 5,
			failingSince: clock.Now(), lastFailedAt: clock.Now(),
		}
		endpoints := fakeEndpoints{target: goneTarget(server.URL), store: store}
		rules := events.DeployedDeliveryRulesForTest(clock.Now)
		clock.Advance(9 * time.Second)

		err := events.DeliverWithRulesForTest(t.Context(), endpoints, store, server.Client(), "prd_test", rules)

		require.ErrorIs(t, err, events.ErrEndpointCoolingDownForTest)
		require.False(t, queue.IsNonRetryable(err))
		require.Zero(t, hits.Load())
		require.EqualValues(t, 5, store.failures, "a skipped call is not a failed call")
	})

	t.Run("After the cooldown one call is made, and its failure doubles the next cooldown", func(t *testing.T) {
		t.Parallel()
		server, hits := countingServer(t, http.StatusServiceUnavailable)
		clock := newFakeClock()
		store := &fakeEndpointStore{
			clock: clock, url: server.URL, failures: 5,
			failingSince: clock.Now(), lastFailedAt: clock.Now(),
		}
		endpoints := fakeEndpoints{target: goneTarget(server.URL), store: store}
		rules := events.DeployedDeliveryRulesForTest(clock.Now)
		clock.Advance(10 * time.Second)

		require.Error(
			t,
			events.DeliverWithRulesForTest(t.Context(), endpoints, store, server.Client(), "prd_test", rules),
		)
		clock.Advance(19 * time.Second)
		err := events.DeliverWithRulesForTest(t.Context(), endpoints, store, server.Client(), "prd_test", rules)
		require.ErrorIs(t, err, events.ErrEndpointCoolingDownForTest)
		clock.Advance(time.Second)
		require.Error(
			t,
			events.DeliverWithRulesForTest(t.Context(), endpoints, store, server.Client(), "prd_test", rules),
		)

		require.EqualValues(t, 2, hits.Load())
		require.EqualValues(t, 7, store.failures)
	})

	t.Run("A success after the cooldown closes the breaker", func(t *testing.T) {
		t.Parallel()
		server, hits := countingServer(t, http.StatusNoContent)
		clock := newFakeClock()
		store := &fakeEndpointStore{
			clock: clock, url: server.URL, failures: 9,
			failingSince: clock.Now(), lastFailedAt: clock.Now(),
		}
		endpoints := fakeEndpoints{target: goneTarget(server.URL), store: store}
		rules := events.DeployedDeliveryRulesForTest(clock.Now)
		clock.Advance(2 * time.Hour)

		require.NoError(
			t,
			events.DeliverWithRulesForTest(t.Context(), endpoints, store, server.Client(), "prd_test", rules),
		)
		require.NoError(
			t,
			events.DeliverWithRulesForTest(t.Context(), endpoints, store, server.Client(), "prd_test", rules),
		)

		require.EqualValues(t, 2, hits.Load())
		require.Zero(t, store.failures)
	})
}

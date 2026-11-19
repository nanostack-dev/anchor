package events_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"anchor/internal/events"

	"github.com/stretchr/testify/require"
)

// fakeEndpoints answers the worker's endpoint lookup with a fixed target.
type fakeEndpoints struct {
	events.EndpointService

	target events.DeliveryTarget
}

func (f fakeEndpoints) DeliveryTarget(context.Context, string) (events.DeliveryTarget, bool, error) {
	return f.target, true, nil
}

// fakeEndpointStore keeps one stored endpoint in memory, with the delivery
// counters the repository keeps.
type fakeEndpointStore struct {
	events.EndpointRepository

	mu       sync.Mutex
	url      string
	failures int32
	deleted  bool
}

func (f *fakeEndpointStore) RecordDeliveryResultInternal(_ context.Context, _, url string, succeeded bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleted || url != f.url {
		return nil
	}
	if succeeded {
		f.failures = 0
	} else {
		f.failures++
	}
	return nil
}

func (f *fakeEndpointStore) DeleteGoneEndpointInternal(
	_ context.Context,
	_, url string,
	minFailed int32,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleted || url != f.url || f.failures < minFailed {
		return false, nil
	}
	f.deleted = true
	return true, nil
}

func (f *fakeEndpointStore) isDeleted() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deleted
}

func goneServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusGone)
	}))
	t.Cleanup(server.Close)
	return server, &hits
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

package events_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/nanostack-dev/pgkit/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/events"
)

func TestEndpointURLRefusesNonPublicAddresses(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"http://127.0.0.1:9/hooks",
		"http://127.10.0.1/hooks",
		"http://10.0.0.5/hooks",
		"http://172.16.4.1/hooks",
		"http://192.168.1.10/hooks",
		"http://100.64.0.1/hooks",
		"http://169.254.169.254/latest",
		"http://0.0.0.0/hooks",
		"http://198.18.0.1/hooks",
		"http://240.0.0.1/hooks",
		"http://224.0.0.1/hooks",
		"http://[::1]/hooks",
		"http://[::]/hooks",
		"http://[fd00::1]/hooks",
		"http://[fe80::1]/hooks",
		"http://[::ffff:127.0.0.1]/hooks",
		"http://[::ffff:10.0.0.1]/hooks",
		"http://[64:ff9b::a00:1]/hooks",
		"http://localhost:8080/hooks",
		"http://LOCALHOST/hooks",
		"http://localhost./hooks",
		"http://api.localhost/hooks",
	} {
		t.Run("A deployment refuses "+raw, func(t *testing.T) {
			t.Parallel()
			assert.Error(t, events.ValidateEndpointURLForTest(raw, events.EndpointURLRules{}))
		})
	}

	t.Run("The test suites accept a loopback endpoint", func(t *testing.T) {
		t.Parallel()
		rules := events.EndpointURLRules{AllowPrivate: true}
		assert.NoError(t, events.ValidateEndpointURLForTest("http://127.0.0.1:9/hooks", rules))
		assert.NoError(t, events.ValidateEndpointURLForTest("http://localhost:9/hooks", rules))
	})

	for _, raw := range []string{
		"https://example.com/hooks",
		"https://203.0.114.10/hooks",
		"https://[2606:4700::1]/hooks",
		"https://api.echopoint.dev/anchor/events",
	} {
		t.Run("A deployment accepts "+raw, func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, events.ValidateEndpointURLForTest(raw, events.EndpointURLRules{RequireHTTPS: true}))
		})
	}
}

func TestDeliveryRefusesNonPublicAddressesAfterDNS(t *testing.T) {
	t.Parallel()

	t.Run("A name that resolves to loopback is refused when the delivery dials", func(t *testing.T) {
		t.Parallel()
		server, hits := countingServer(t, http.StatusOK)
		target := goneTarget(strings.Replace(server.URL, "127.0.0.1", "localhost", 1))
		store := &fakeEndpointStore{url: target.URL}

		err := events.DeliverForTest(context.Background(), fakeEndpoints{target: target}, store,
			events.DeliveryClientForTest(false), "prd_test")

		require.ErrorIs(t, err, events.ErrNonPublicAddressForTest)
		require.True(t, queue.IsNonRetryable(err), "a refused address fails the job without retries")
		require.Zero(t, hits.Load())
		require.EqualValues(t, 1, store.failures)
	})

	t.Run("The test suites deliver to a loopback endpoint", func(t *testing.T) {
		t.Parallel()
		server, hits := countingServer(t, http.StatusOK)
		target := goneTarget(server.URL)
		store := &fakeEndpointStore{url: target.URL}

		err := events.DeliverForTest(context.Background(), fakeEndpoints{target: target}, store,
			events.DeliveryClientForTest(true), "prd_test")

		require.NoError(t, err)
		require.EqualValues(t, 1, hits.Load())
	})
}

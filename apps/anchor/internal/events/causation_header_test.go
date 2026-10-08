package events_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/events"
)

func TestCausationHeader_RoundTrips(t *testing.T) {
	sent := events.Causation{Depth: 2, WorkflowIDs: []string{"wf_a", "wf_b"}}

	received, ok := events.DecodeCausationHeader(events.EncodeCausationHeader(sent))

	require.True(t, ok)
	assert.Equal(t, sent, received)
}

func TestCausationHeader_IgnoresAnUnreadableValue(t *testing.T) {
	for _, value := range []string{"", "not base64!", "e30x", events.EncodeCausationHeader(events.Causation{Depth: -1})} {
		_, ok := events.DecodeCausationHeader(value)
		assert.False(t, ok, value)
	}
}

func TestCausationMiddleware_PutsTheHeaderIntoTheContext(t *testing.T) {
	var seen events.Causation
	handler := events.CausationMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = events.CausationFrom(r.Context())
	}))
	request := httptest.NewRequest(http.MethodPost, "/v1/products/p/organizations", nil)
	request.Header.Set(events.CausationHeader,
		events.EncodeCausationHeader(events.Causation{Depth: 1, WorkflowIDs: []string{"wf_a"}}))

	handler.ServeHTTP(httptest.NewRecorder(), request)

	assert.Equal(t, events.Causation{Depth: 1, WorkflowIDs: []string{"wf_a"}}, seen)
}

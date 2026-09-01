package itdsl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
	"github.com/stretchr/testify/require"
)

const (
	eventSinkWaitTimeout  = 20 * time.Second
	eventSinkPollInterval = 50 * time.Millisecond
)

// EventSink is an httptest receiver that verifies Standard Webhooks signatures
// through [anchorsdk.WebhookHandler] and records the typed events.
type EventSink struct {
	URL    string
	Secret string

	t        *testing.T
	mu       sync.Mutex
	received []anchorsdk.Event
}

// CaptureEvents configures this product's event endpoint to a local sink.
func (tp *ProductContext) CaptureEvents() *EventSink {
	tp.testingContext.Helper()
	sink := newEventSink(tp.testingContext)
	tp.attachEventSink(sink)
	return sink
}

func newEventSink(t *testing.T) *EventSink {
	t.Helper()
	sink := &EventSink{t: t}
	server := httptest.NewServer(http.HandlerFunc(sink.serve))
	t.Cleanup(server.Close)
	sink.URL = server.URL
	return sink
}

func (s *EventSink) serve(writer http.ResponseWriter, request *http.Request) {
	s.mu.Lock()
	secret := s.Secret
	s.mu.Unlock()
	if secret == "" {
		http.Error(writer, "signing secret not minted yet", http.StatusInternalServerError)
		return
	}
	handler, err := anchorsdk.Events(secret)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	handler.OnAny(func(_ context.Context, event anchorsdk.Event) error {
		s.mu.Lock()
		s.received = append(s.received, event)
		s.mu.Unlock()
		return nil
	})
	handler.ServeHTTP(writer, request)
}

func (tp *ProductContext) attachEventSink(sink *EventSink) {
	tp.testingContext.Helper()
	ctx := context.Background()
	got, err := tp.OwnerAuthenticatedClient().GetProductWithResponse(ctx, tp.ProductID)
	require.NoError(tp.testingContext, err)
	require.Equal(tp.testingContext, http.StatusOK, got.StatusCode())
	require.NotNil(tp.testingContext, got.JSON200)

	updated, updateErr := tp.OwnerAuthenticatedClient().UpdateProductWithResponse(
		ctx,
		tp.ProductID,
		nanostackClient.UpdateProductJSONRequestBody{
			Name:        got.JSON200.Name,
			Description: got.JSON200.Description,
			Config: &nanostackClient.ProductConfigRequest{
				OrganizationApiKeys: &nanostackClient.ProductOrganizationAPIKeysConfigRequest{
					Prefix: got.JSON200.Config.OrganizationApiKeys.Prefix,
				},
				Events: &nanostackClient.ProductEventsConfigRequest{
					EndpointUrl: &sink.URL,
				},
			},
		},
	)
	require.NoError(tp.testingContext, updateErr)
	require.Equal(tp.testingContext, http.StatusOK, updated.StatusCode())
	require.NotNil(tp.testingContext, updated.JSON200)
	require.NotNil(tp.testingContext, updated.JSON200.Config.Events)
	require.NotNil(tp.testingContext, updated.JSON200.Config.Events.SigningSecret)
	sink.mu.Lock()
	sink.Secret = *updated.JSON200.Config.Events.SigningSecret
	sink.mu.Unlock()
}

// WaitFor waits until a recorded event matches type and every supplied field.
func (s *EventSink) WaitFor(eventType string, fields map[string]string) anchorsdk.Event {
	s.t.Helper()
	var found anchorsdk.Event
	require.Eventually(s.t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, event := range s.received {
			if event.Type != eventType {
				continue
			}
			if eventMatches(event, fields) {
				found = event
				return true
			}
		}
		return false
	}, eventSinkWaitTimeout, eventSinkPollInterval)
	return found
}

func (s *EventSink) Count(eventType string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, event := range s.received {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func eventMatches(event anchorsdk.Event, fields map[string]string) bool {
	for key, want := range fields {
		if event.Field(key) != want {
			return false
		}
	}
	return true
}

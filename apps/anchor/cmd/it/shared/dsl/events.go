package itdsl

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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
	signingSecretBytes    = 32
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
	secret := mustSigningSecret(t)
	sink := &EventSink{t: t, Secret: secret}
	handler, err := anchorsdk.Events(secret)
	require.NoError(t, err)
	handler.OnAny(func(_ context.Context, event anchorsdk.Event) error {
		sink.mu.Lock()
		sink.received = append(sink.received, event)
		sink.mu.Unlock()
		return nil
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	sink.URL = server.URL
	return sink
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
					EndpointUrl:   &sink.URL,
					SigningSecret: &sink.Secret,
				},
			},
		},
	)
	require.NoError(tp.testingContext, updateErr)
	require.Equal(tp.testingContext, http.StatusOK, updated.StatusCode())
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

func eventMatches(event anchorsdk.Event, fields map[string]string) bool {
	for key, want := range fields {
		if event.Field(key) != want {
			return false
		}
	}
	return true
}

func mustSigningSecret(t *testing.T) string {
	t.Helper()
	key := make([]byte, signingSecretBytes)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return "whsec_" + base64.StdEncoding.EncodeToString(key)
}

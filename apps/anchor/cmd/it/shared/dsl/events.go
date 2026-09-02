package itdsl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	maxEventSinkBodyBytes = 1 << 20
)

type recordedDelivery struct {
	event    anchorsdk.Event
	delivery StandardWebhookDelivery
}

type EventSink struct {
	URL    string
	Secret string

	t        *testing.T
	mu       sync.Mutex
	received []recordedDelivery
}

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
	t.Cleanup(sink.assertAllDeliveries)
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
	body, err := io.ReadAll(io.LimitReader(request.Body, maxEventSinkBodyBytes+1))
	if err != nil || len(body) > maxEventSinkBodyBytes {
		http.Error(writer, "invalid body", http.StatusBadRequest)
		return
	}
	headers := request.Header.Clone()
	delivery := StandardWebhookDelivery{
		Method:  request.Method,
		Headers: headers,
		Body:    body,
	}
	if !standardWebhookSignatureMatches(secret, headers, body) {
		http.Error(writer, "invalid signature", http.StatusBadRequest)
		return
	}
	event, err := decodeRecordedEvent(headers.Get(webhookIDHeader), body)
	if err != nil {
		http.Error(writer, "invalid payload", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.received = append(s.received, recordedDelivery{event: event, delivery: delivery})
	s.mu.Unlock()
	writer.WriteHeader(http.StatusOK)
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

func (s *EventSink) WaitFor(eventType string, fields map[string]string) anchorsdk.Event {
	s.t.Helper()
	var found anchorsdk.Event
	require.Eventually(s.t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, recorded := range s.received {
			if recorded.event.Type != eventType {
				continue
			}
			if eventMatches(recorded.event, fields) {
				found = recorded.event
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
	for _, recorded := range s.received {
		if recorded.event.Type == eventType {
			count++
		}
	}
	return count
}

func (s *EventSink) assertAllDeliveries() {
	s.t.Helper()
	s.mu.Lock()
	received := append([]recordedDelivery{}, s.received...)
	secret := s.Secret
	s.mu.Unlock()
	for _, recorded := range received {
		AssertStandardWebhook(s.t, secret, recorded.delivery)
	}
}

func eventMatches(event anchorsdk.Event, fields map[string]string) bool {
	for key, want := range fields {
		if event.Field(key) != want {
			return false
		}
	}
	return true
}

func decodeRecordedEvent(msgID string, body []byte) (anchorsdk.Event, error) {
	var envelope struct {
		Type      string          `json:"type"`
		Timestamp string          `json:"timestamp"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return anchorsdk.Event{}, err
	}
	if envelope.Type == "" {
		return anchorsdk.Event{}, errors.New("missing type")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, envelope.Timestamp)
	if err != nil {
		timestamp, err = time.Parse(time.RFC3339, envelope.Timestamp)
		if err != nil {
			return anchorsdk.Event{}, err
		}
	}
	return anchorsdk.Event{
		ID:        msgID,
		Type:      envelope.Type,
		Timestamp: timestamp,
		Data:      envelope.Data,
	}, nil
}

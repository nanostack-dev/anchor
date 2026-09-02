package itdsl

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
	"github.com/stretchr/testify/require"
)

const (
	eventSinkWaitTimeout    = 20 * time.Second
	eventSinkPollInterval   = 50 * time.Millisecond
	eventSinkTimestampSkew  = 5 * time.Minute
	eventIDPrefix           = "pevt_"
	webhookIDHeader         = "Webhook-Id"
	webhookTimestampHeader  = "Webhook-Timestamp"
	webhookSignatureHeader  = "Webhook-Signature"
	webhookSignatureVersion = "v1,"
	signingSecretPrefix     = "whsec_"
	signingSecretBytes      = 32
	maxEventSinkBodyBytes   = 1 << 20
)

type recordedDelivery struct {
	event   anchorsdk.Event
	method  string
	headers http.Header
	body    []byte
}

type eventEnvelope struct {
	Type      string            `json:"type"`
	Timestamp string            `json:"timestamp"`
	Data      map[string]string `json:"data"`
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
	if !standardWebhookSignatureMatches(secret, headers, body) {
		http.Error(writer, "invalid signature", http.StatusBadRequest)
		return
	}
	msgID := headers.Get(webhookIDHeader)
	event, err := decodeRecordedEvent(msgID, body)
	if err != nil {
		http.Error(writer, "invalid payload", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.received = append(s.received, recordedDelivery{
		event:   event,
		method:  request.Method,
		headers: headers,
		body:    body,
	})
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
	var found recordedDelivery
	require.Eventually(s.t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, delivery := range s.received {
			if delivery.event.Type != eventType {
				continue
			}
			if eventMatches(delivery.event, fields) {
				found = delivery
				return true
			}
		}
		return false
	}, eventSinkWaitTimeout, eventSinkPollInterval)
	s.assertStandardWebhook(found, eventType, fields)
	return found.event
}

func (s *EventSink) Count(eventType string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, delivery := range s.received {
		if delivery.event.Type == eventType {
			count++
		}
	}
	return count
}

func (s *EventSink) assertStandardWebhook(
	delivery recordedDelivery, eventType string, fields map[string]string,
) {
	s.t.Helper()
	require.Equal(s.t, http.MethodPost, delivery.method)
	require.Equal(s.t, "application/json", delivery.headers.Get("Content-Type"))

	msgID := delivery.headers.Get(webhookIDHeader)
	timestampHeader := delivery.headers.Get(webhookTimestampHeader)
	signatureHeader := delivery.headers.Get(webhookSignatureHeader)
	require.NotEmpty(s.t, msgID)
	require.NotEmpty(s.t, timestampHeader)
	require.NotEmpty(s.t, signatureHeader)
	require.True(s.t, strings.HasPrefix(msgID, eventIDPrefix), "webhook-id %q", msgID)
	require.Equal(s.t, msgID, delivery.event.ID)

	unixSeconds, err := strconv.ParseInt(timestampHeader, 10, 64)
	require.NoError(s.t, err)
	headerTime := time.Unix(unixSeconds, 0)
	skew := time.Since(headerTime)
	require.LessOrEqual(s.t, skew, eventSinkTimestampSkew)
	require.GreaterOrEqual(s.t, skew, -eventSinkTimestampSkew)

	require.True(
		s.t,
		standardWebhookSignatureMatches(s.Secret, delivery.headers, delivery.body),
		"signature must match the minted secret over Webhook-Id, Webhook-Timestamp, and body",
	)
	require.False(
		s.t,
		standardWebhookSignatureMatches(otherSigningSecret(), delivery.headers, delivery.body),
		"signature must not match a different secret",
	)
	tampered := append(append([]byte{}, delivery.body...), 'x')
	require.False(
		s.t,
		standardWebhookSignatureMatches(s.Secret, delivery.headers, tampered),
		"signature must not match a tampered body",
	)

	var envelope eventEnvelope
	require.NoError(s.t, json.Unmarshal(delivery.body, &envelope))
	require.Equal(s.t, eventType, envelope.Type)
	_, timestampErr := time.Parse(time.RFC3339Nano, envelope.Timestamp)
	if timestampErr != nil {
		_, timestampErr = time.Parse(time.RFC3339, envelope.Timestamp)
	}
	require.NoError(s.t, timestampErr)
	require.NotNil(s.t, envelope.Data)
	for key, want := range fields {
		require.Equal(s.t, want, envelope.Data[key], key)
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

func standardWebhookSignatureMatches(secret string, headers http.Header, body []byte) bool {
	msgID := headers.Get(webhookIDHeader)
	timestampHeader := headers.Get(webhookTimestampHeader)
	signatures := headers.Get(webhookSignatureHeader)
	if msgID == "" || timestampHeader == "" || signatures == "" {
		return false
	}
	unixSeconds, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return false
	}
	key, err := decodeSigningSecret(secret)
	if err != nil {
		return false
	}
	expected := signStandardWebhook(key, msgID, unixSeconds, body)
	for candidate := range strings.SplitSeq(signatures, " ") {
		if hmac.Equal([]byte(candidate), []byte(expected)) {
			return true
		}
	}
	return false
}

func signStandardWebhook(key []byte, msgID string, unixSeconds int64, body []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = fmt.Fprintf(mac, "%s.%d.", msgID, unixSeconds)
	_, _ = mac.Write(body)
	return webhookSignatureVersion + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func decodeSigningSecret(secret string) ([]byte, error) {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return nil, errors.New("empty secret")
	}
	payload := strings.TrimPrefix(trimmed, signingSecretPrefix)
	key, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(key) == 0 {
		return nil, errors.New("invalid secret")
	}
	return key, nil
}

func otherSigningSecret() string {
	return signingSecretPrefix + base64.StdEncoding.EncodeToString(
		bytes.Repeat([]byte{0x42}, signingSecretBytes),
	)
}

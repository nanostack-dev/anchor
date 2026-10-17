package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"anchor/internal/api"

	"github.com/stretchr/testify/require"
)

type trackedBody struct {
	io.Reader
	closed bool
}

func (body *trackedBody) Close() error {
	body.closed = true
	return nil
}

func TestWebhookPayloadPreservesDecoderInput(t *testing.T) {
	t.Parallel()
	payload := "{\n \"type\": \"user.created\", \"data\": {\"z\":2,\"a\":1} }\n"
	var received []byte
	var readErr error
	handler := api.WebhookPayloadMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received, readErr = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost,
		"/v1/products/prd_test/integrations/webhooks/clerk", strings.NewReader(payload))
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)
	require.NoError(t, readErr)
	require.Equal(t, payload, string(received))
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(received, &decoded))
	require.Equal(t, "user.created", decoded["type"])
}

func TestWebhookPayloadLeavesOtherRequestsUntouched(t *testing.T) {
	t.Parallel()
	original := &trackedBody{Reader: strings.NewReader("ordinary body")}
	var received io.ReadCloser
	handler := api.WebhookPayloadMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received = r.Body
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	request := httptest.NewRequest(http.MethodPost, "/v1/products", nil)
	request.Body = original
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)
	require.Same(t, original, received)
	require.False(t, original.closed)
	body, err := io.ReadAll(received)
	require.NoError(t, err)
	require.Equal(t, "ordinary body", string(body))
}

func TestWebhookPayloadRejectsOversizedBodyBeforeDispatch(t *testing.T) {
	t.Parallel()
	called := false
	handler := api.WebhookPayloadMiddleware(
		http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { called = true }),
	)
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/products/prd_test/integrations/webhooks/clerk",
		strings.NewReader(strings.Repeat(" ", (1<<20)+1)),
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.False(t, called)
}

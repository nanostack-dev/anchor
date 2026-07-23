package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// webhookReceiver is a WireMock container standing in for a customer's endpoint.
//
// A real HTTP server is what makes these tests worth running: the delivery path
// signs, dials, POSTs and reads a bounded response, and only an actual receiver
// exercises the headers end to end. Stubs are registered over WireMock's admin
// API so no testdata mapping files are needed.
type webhookReceiver struct {
	baseURL string
	client  *http.Client
}

const (
	wireMockImage        = "wiremock/wiremock:3.3.1"
	wireMockPort         = "8080/tcp"
	wireMockStartTimeout = 60 * time.Second
	receiverPath         = "/hooks/anchor"
)

// startWebhookReceiver boots WireMock and returns a handle scoped to the test.
func startWebhookReceiver(t *testing.T) *webhookReceiver {
	t.Helper()

	ctx := context.Background()
	container, err := testcontainers.GenericContainer(
		ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        wireMockImage,
				ExposedPorts: []string{wireMockPort},
				WaitingFor: wait.ForHTTP("/__admin/health").
					WithPort(wireMockPort).
					WithStartupTimeout(wireMockStartTimeout),
			},
			Started: true,
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, testcontainers.TerminateContainer(container))
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, wireMockPort)
	require.NoError(t, err)

	return &webhookReceiver{
		baseURL: "http://" + net.JoinHostPort(host, port.Port()),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// TargetURL is the URL a webhook endpoint should be pointed at.
func (r *webhookReceiver) TargetURL() string {
	return r.baseURL + receiverPath
}

// RespondWith registers the status code the receiver answers with.
func (r *webhookReceiver) RespondWith(t *testing.T, statusCode int, body string) {
	t.Helper()

	r.resetMappings(t)
	r.postAdmin(t, "/__admin/mappings", map[string]any{
		"request": map[string]any{
			"method": http.MethodPost,
			"url":    receiverPath,
		},
		"response": map[string]any{
			"status": statusCode,
			"body":   body,
		},
	})
}

// RecordedRequests returns every request the receiver has seen on its path.
func (r *webhookReceiver) RecordedRequests(t *testing.T) []recordedRequest {
	t.Helper()

	raw := r.postAdmin(t, "/__admin/requests/find", map[string]any{
		"method": http.MethodPost,
		"url":    receiverPath,
	})

	var found struct {
		Requests []recordedRequest `json:"requests"`
	}
	require.NoError(t, json.Unmarshal(raw, &found))

	return found.Requests
}

type recordedRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

func (r *webhookReceiver) resetMappings(t *testing.T) {
	t.Helper()
	r.postAdmin(t, "/__admin/reset", nil)
}

func (r *webhookReceiver) postAdmin(t *testing.T, path string, payload any) []byte {
	t.Helper()

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, r.baseURL+path, body,
	)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")

	response, err := r.client.Do(request)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, response.Body.Close())
	}()

	responseBody, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Lessf(
		t, response.StatusCode, http.StatusBadRequest,
		"wiremock admin call %s failed: %s", path, string(responseBody),
	)

	return responseBody
}

// header reads a recorded header case-insensitively, because HTTP/2 lowercases
// header names and HTTP/1.1 does not.
func (r recordedRequest) header(name string) string {
	for key, value := range r.Headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}

	return ""
}

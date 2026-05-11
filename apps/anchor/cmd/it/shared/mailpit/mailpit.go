// Package mailpit provides a testcontainers helper that spins up a mailpit
// container for SMTP integration tests. Mailpit (axllent/mailpit) is a
// developer-friendly SMTP catch-all server that exposes a REST API on port
// 8025 — perfect for asserting that a message we just dispatched actually
// reached the wire.
//
// Usage from a CT:
//
//	mp := mailpit.Start(t)
//	defer mp.Stop(t)
//	// dial mp.SMTPHost:mp.SMTPPort, send a message
//	msgs := mp.Messages(t)
//	require.Equal(t, 1, msgs.Total)
package mailpit

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	smtpContainerPort = "1025/tcp"
	httpContainerPort = "8025/tcp"

	pollInterval   = 100 * time.Millisecond
	startupTimeout = 2 * time.Minute
)

// Mailpit holds the connection coordinates for a running mailpit container.
type Mailpit struct {
	container testcontainers.Container

	SMTPHost string
	SMTPPort int
	APIBase  string // e.g. "http://localhost:54321"
}

// Start launches a mailpit container and returns once both SMTP and HTTP
// endpoints are reachable. The image is pinned to the upstream stable tag.
func Start(t *testing.T) *Mailpit {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "axllent/mailpit:v1.20",
		ExposedPorts: []string{smtpContainerPort, httpContainerPort},
		// Accept SMTP-AUTH PLAIN/LOGIN with any credentials over plaintext so
		// the CT exercises the real authentication path. Without these flags
		// mailpit refuses to advertise AUTH on a non-TLS listener and any
		// SMTP client that issues AUTH (the way every hosted provider works
		// in production) would fail.
		Env: map[string]string{
			"MP_SMTP_AUTH_ACCEPT_ANY":     "true",
			"MP_SMTP_AUTH_ALLOW_INSECURE": "true",
		},
		WaitingFor: wait.ForListeningPort(smtpContainerPort).WithStartupTimeout(startupTimeout),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("mailpit: start container: %v", err)
	}

	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(ctx)
		t.Fatalf("mailpit: host lookup: %v", err)
	}
	smtpPort, err := c.MappedPort(ctx, smtpContainerPort)
	if err != nil {
		_ = c.Terminate(ctx)
		t.Fatalf("mailpit: smtp port lookup: %v", err)
	}
	apiPort, err := c.MappedPort(ctx, httpContainerPort)
	if err != nil {
		_ = c.Terminate(ctx)
		t.Fatalf("mailpit: api port lookup: %v", err)
	}

	smtpPortInt, err := strconv.Atoi(smtpPort.Port())
	if err != nil {
		_ = c.Terminate(ctx)
		t.Fatalf("mailpit: invalid smtp port %q: %v", smtpPort.Port(), err)
	}

	apiBase := "http://" + net.JoinHostPort(host, apiPort.Port())
	waitForAPIReady(t, apiBase)

	return &Mailpit{
		container: c,
		SMTPHost:  host,
		SMTPPort:  smtpPortInt,
		APIBase:   apiBase,
	}
}

func waitForAPIReady(t *testing.T, apiBase string) {
	t.Helper()

	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			apiBase+"/api/v1/messages",
			nil,
		)
		if err != nil {
			t.Fatalf("mailpit: build readiness request: %v", err)
		}

		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}

		time.Sleep(pollInterval)
	}

	t.Fatalf("mailpit: API not ready within %s", startupTimeout)
}

// Stop terminates the container; safe to call from defer.
func (m *Mailpit) Stop(t *testing.T) {
	t.Helper()
	if m == nil || m.container == nil {
		return
	}
	if err := m.container.Terminate(context.Background()); err != nil {
		t.Logf("mailpit: terminate container: %v", err)
	}
}

// Reset clears all messages held by mailpit. Safe to call between
// subtests so each scenario starts from a clean slate.
func (m *Mailpit) Reset(t *testing.T) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, m.APIBase+"/api/v1/messages", nil)
	if err != nil {
		t.Fatalf("mailpit: build reset request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("mailpit: reset request: %v", err)
	}
	_ = resp.Body.Close()
}

// MessageSummary mirrors the subset of mailpit's /api/v1/messages payload
// we care about. Field names track mailpit's JSON exactly.
type MessageSummary struct {
	ID      string           `json:"ID"`
	From    AddressSummary   `json:"From"`
	To      []AddressSummary `json:"To"`
	Subject string           `json:"Subject"`
	Snippet string           `json:"Snippet"`
}

type AddressSummary struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

type listResponse struct {
	Total    int              `json:"total"`
	Messages []MessageSummary `json:"messages"`
}

// Messages returns every message currently held by mailpit (newest first).
func (m *Mailpit) Messages(t *testing.T) []MessageSummary {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, m.APIBase+"/api/v1/messages", nil)
	if err != nil {
		t.Fatalf("mailpit: build list messages request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("mailpit: list messages: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mailpit: list messages: status %d", resp.StatusCode)
	}
	var lr listResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&lr); decodeErr != nil {
		t.Fatalf("mailpit: decode list response: %v", decodeErr)
	}
	return lr.Messages
}

// WaitForMessage polls mailpit until at least one message is available or
// the timeout expires. Returns the first message — convenient for the
// common single-send assertion.
func (m *Mailpit) WaitForMessage(t *testing.T, timeout time.Duration) MessageSummary {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msgs := m.Messages(t)
		if len(msgs) > 0 {
			return msgs[0]
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("mailpit: no message received within %s", timeout)
	return MessageSummary{}
}

// MessageBody fetches the rendered HTML + plain-text body of a single
// message by ID via mailpit's /api/v1/message/{id} endpoint.
type MessageBody struct {
	ID      string           `json:"ID"`
	From    AddressSummary   `json:"From"`
	To      []AddressSummary `json:"To"`
	Subject string           `json:"Subject"`
	HTML    string           `json:"HTML"`
	Text    string           `json:"Text"`
}

func (m *Mailpit) MessageByID(t *testing.T, id string) MessageBody {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, m.APIBase+"/api/v1/message/"+id, nil)
	if err != nil {
		t.Fatalf("mailpit: build fetch request for message %s: %v", id, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("mailpit: fetch message %s: %v", id, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mailpit: fetch message %s: status %d", id, resp.StatusCode)
	}
	var body MessageBody
	if decodeErr := json.NewDecoder(resp.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("mailpit: decode message %s: %v", id, decodeErr)
	}
	return body
}

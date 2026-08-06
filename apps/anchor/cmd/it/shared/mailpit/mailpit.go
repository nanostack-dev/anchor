// Package mailpit provides a testcontainers helper that spins up a mailpit
// container for SMTP integration tests. Mailpit (axllent/mailpit) is a
// developer-friendly SMTP catch-all server that exposes a REST API on port
// 8025 — perfect for asserting that a message we just dispatched actually
// reached the wire.
//
// One container is shared by every test in the process, the same way the CT
// suite shares its postgres and redis containers. A full `go test ./cmd/it/...`
// runs one binary per package concurrently, each starting its own
// postgres + redis; a mailpit container per test function on top of that
// saturates the Docker daemon until the readiness wait times out. Sharing keeps
// it to one.
//
// Usage from a CT:
//
//	mp := mailpit.Shared(t)   // messages are cleared for this test
//	// dial mp.SMTPHost:mp.SMTPPort, send a message
//	msgs := mp.Messages(t)
//	require.Len(t, msgs, 1)
//
// The owning package releases it from TestMain:
//
//	itshared.RunTestMain(m, itshared.TestConfig{AfterRun: mailpit.StopShared, ...})
package mailpit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	smtpContainerPort = "1025/tcp"
	httpContainerPort = "8025/tcp"

	pollInterval = 100 * time.Millisecond
	// startupTimeout budgets the wait for Docker to publish the port mapping.
	// That wait polls the daemon, so a daemon busy starting the rest of the CT
	// suite's containers burns the budget on inspect calls rather than on the
	// container actually being slow.
	startupTimeout = 3 * time.Minute
	// mappedPortPollInterval throttles that same poll loop. Every tick costs two
	// Docker round trips, so polling faster than this only adds load to the
	// daemon we are already waiting on.
	mappedPortPollInterval = 500 * time.Millisecond
	// readyTimeout bounds the endpoint probes we run ourselves. These talk to
	// mailpit directly and never touch the Docker daemon, so they need far less
	// slack than the port-mapping wait.
	readyTimeout = 30 * time.Second
	dialTimeout  = 2 * time.Second
	// startAttempts retries a failed start once with a fresh budget, which
	// clears a transient daemon stall. Keep the product of attempts and
	// startupTimeout well under `go test`'s 10 minute default timeout.
	startAttempts = 2
)

// Mailpit holds the connection coordinates for a running mailpit container.
type Mailpit struct {
	container testcontainers.Container

	SMTPHost string
	SMTPPort int
	APIBase  string // e.g. "http://localhost:54321"
}

// sharedContainer holds the outcome of the one start attempt a test process
// makes. Keeping the failure next to the container lets every later caller
// report the same reason instead of re-running a start that already failed.
type sharedContainer struct {
	mp     *Mailpit
	reason error
}

var (
	sharedOnce sync.Once       //nolint:gochecknoglobals // process-wide container, started on first use
	sharedMu   sync.Mutex      //nolint:gochecknoglobals // guards the container against a concurrent StopShared
	sharedMP   sharedContainer //nolint:gochecknoglobals // process-wide container, started on first use
)

// Shared returns the process-wide mailpit container, starting it on first use,
// and clears any messages left behind by an earlier test so the caller starts
// from an empty inbox. Tests in a package run sequentially unless they opt into
// t.Parallel, which no mailpit-backed test does — a parallel test would see
// another test's reset.
func Shared(t *testing.T) *Mailpit {
	t.Helper()

	sharedOnce.Do(func() {
		mp, err := start()
		sharedMu.Lock()
		defer sharedMu.Unlock()
		sharedMP = sharedContainer{mp: mp, reason: err}
	})

	sharedMu.Lock()
	current := sharedMP
	sharedMu.Unlock()

	if current.reason != nil {
		t.Fatalf("mailpit: start shared container: %v", current.reason)
	}
	if current.mp == nil {
		t.Fatal("mailpit: shared container already stopped")
	}

	current.mp.Reset(t)
	return current.mp
}

// StopShared terminates the process-wide container. Wire it into TestMain
// teardown via itshared.TestConfig.AfterRun — a defer in TestMain never runs
// because RunTestMain exits the process. No-op when Shared was never called.
func StopShared() {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if sharedMP.mp == nil {
		return
	}
	terminate(sharedMP.mp.container)
	sharedMP.mp = nil
}

// start launches the container, retrying once so a daemon that stalled during
// the first attempt gets a fresh budget rather than failing the package.
func start() (*Mailpit, error) {
	var lastErr error
	for range startAttempts {
		mp, err := startOnce()
		if err == nil {
			return mp, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("%d attempts: %w", startAttempts, lastErr)
}

// startOnce launches a mailpit container and returns once both SMTP and HTTP
// endpoints are reachable. The image is pinned to the upstream stable tag.
// Every failure path terminates the container so a retry does not leave one
// running on the daemon we are contending with.
func startOnce() (*Mailpit, error) {
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
		// Wait only for the port mapping to be published. ForListeningPort adds
		// an external dial and an in-container /bin/sh probe, and the probe costs
		// another Docker round trip per poll on a daemon that is already the
		// bottleneck. The dial and the API check below establish readiness
		// without involving Docker at all.
		WaitingFor: wait.ForMappedPort(smtpContainerPort).
			WithStartupTimeout(startupTimeout).
			WithPollInterval(mappedPortPollInterval),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		// GenericContainer returns a non-nil container alongside an error when
		// creation succeeded and the readiness wait failed.
		terminate(c)
		return nil, fmt.Errorf("start container: %w", err)
	}

	mp, err := resolveEndpoints(ctx, c)
	if err != nil {
		terminate(c)
		return nil, err
	}

	if smtpErr := waitForSMTPReady(ctx, mp.SMTPHost, mp.SMTPPort); smtpErr != nil {
		terminate(c)
		return nil, smtpErr
	}
	if apiErr := waitForAPIReady(ctx, mp.APIBase); apiErr != nil {
		terminate(c)
		return nil, apiErr
	}

	return mp, nil
}

// resolveEndpoints reads the published host and ports off a started container.
func resolveEndpoints(ctx context.Context, c testcontainers.Container) (*Mailpit, error) {
	host, err := c.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("host lookup: %w", err)
	}
	smtpPort, err := c.MappedPort(ctx, smtpContainerPort)
	if err != nil {
		return nil, fmt.Errorf("smtp port lookup: %w", err)
	}
	apiPort, err := c.MappedPort(ctx, httpContainerPort)
	if err != nil {
		return nil, fmt.Errorf("api port lookup: %w", err)
	}
	smtpPortInt, err := strconv.Atoi(smtpPort.Port())
	if err != nil {
		return nil, fmt.Errorf("invalid smtp port %q: %w", smtpPort.Port(), err)
	}

	return &Mailpit{
		container: c,
		SMTPHost:  host,
		SMTPPort:  smtpPortInt,
		APIBase:   "http://" + net.JoinHostPort(host, apiPort.Port()),
	}, nil
}

// waitForSMTPReady dials the mapped SMTP port until mailpit accepts a
// connection. This replaces the external check ForListeningPort would run,
// without the per-poll container state lookup that check makes.
func waitForSMTPReady(ctx context.Context, host string, port int) error {
	dialer := net.Dialer{Timeout: dialTimeout}
	address := net.JoinHostPort(host, strconv.Itoa(port))

	var lastErr error
	deadline := time.Now().Add(readyTimeout)
	for time.Now().Before(deadline) {
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("smtp %s not reachable within %s: %w", address, readyTimeout, lastErr)
}

func waitForAPIReady(ctx context.Context, apiBase string) error {
	client := &http.Client{Timeout: dialTimeout}

	var lastErr error
	deadline := time.Now().Add(readyTimeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/api/v1/messages", nil)
		if err != nil {
			return fmt.Errorf("build readiness request: %w", err)
		}

		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		time.Sleep(pollInterval)
	}

	return fmt.Errorf("API %s not ready within %s: %w", apiBase, readyTimeout, lastErr)
}

// terminate stops a container, tolerating a nil handle so failure paths can
// call it unconditionally. There is no *testing.T on the shared start path, so
// a termination failure is reported on stderr.
func terminate(c testcontainers.Container) {
	if c == nil {
		return
	}
	if err := c.Terminate(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "mailpit: terminate container: %v\n", err)
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

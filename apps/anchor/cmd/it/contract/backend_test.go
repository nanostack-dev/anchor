//go:build contract

package contract_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

// The suite stops at the first request that fails because the server or its
// database is not reachable: every later request would fail the same way, and
// each flow would then wait out its webhook_wait timeout (30 s plus a 3 s
// settle) before failing. The request that finds it writes no golden either.

// backendUnavailable matches a database that is starting, stopping or gone
// (SQLSTATE class 57P0: admin_shutdown, crash_shutdown, cannot_connect_now)
// and a refused connection, in an error the server returned or logged.
var backendUnavailable = regexp.MustCompile(
	`\b57P0[123]\b|connection refused|the database system is (?:starting up|shutting down|in recovery mode)`)

var backend struct {
	mu   sync.Mutex
	down string
}

// backendDown returns why the backend was found unavailable, or "".
func backendDown() string {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	return backend.down
}

// checkBackend reports whether a request failed because the backend is
// unavailable, and records the first such failure for the rest of the run.
// err is the transport error; with none, a 5xx response is checked with the
// log lines the server wrote for it.
func checkBackend(requestID string, resp *response, lines []map[string]any, err error) (string, bool) {
	var evidence string
	switch {
	case err != nil:
		if !errors.Is(err, syscall.ECONNREFUSED) && !backendUnavailable.MatchString(err.Error()) {
			return "", false
		}
		evidence = err.Error()
	case resp != nil && resp.status >= http.StatusInternalServerError:
		raw, _ := json.Marshal(lines)
		text := string(resp.body) + "\n" + string(raw)
		match := backendUnavailable.FindStringIndex(text)
		if match == nil {
			return "", false
		}
		const around = 160
		start, end := max(match[0]-around, 0), min(match[1]+around, len(text))
		evidence = fmt.Sprintf("status %d: ...%s...", resp.status, strings.TrimSpace(text[start:end]))
	default:
		return "", false
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.down == "" {
		backend.down = fmt.Sprintf("backend unavailable at %s: %s", requestID, evidence)
	}
	return backend.down, true
}

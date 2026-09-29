// Package logcapture records the JSON log lines the in-process Anchor server
// writes, so a component test can assert that a failure was logged.
//
// The application builds its logger on os.Stdout when it starts, so Install
// must run in TestMain before itshared.RunTestMain. Every line still reaches
// the original stdout.
package logcapture

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	maxLineBytes = 1 << 20
	waitTimeout  = 10 * time.Second
	pollInterval = 50 * time.Millisecond
)

// Entry is one JSON log line and the raw text it came from.
type Entry struct {
	Raw    string
	Fields map[string]any
}

func (e Entry) String(field string) string {
	value, _ := e.Fields[field].(string)
	return value
}

var (
	mu      sync.Mutex //nolint:gochecknoglobals // process-wide capture, installed once by TestMain
	entries []Entry    //nolint:gochecknoglobals // process-wide capture, installed once by TestMain
)

func Install() {
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(fmt.Sprintf("logcapture: create pipe: %v", err))
	}
	os.Stdout = writer //nolint:reassign // the application builds its logger on os.Stdout

	go func() {
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, maxLineBytes), maxLineBytes)
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = fmt.Fprintln(original, line)
			record(line)
		}
	}()
}

func record(line string) {
	entry := Entry{Raw: line}
	var fields map[string]any
	if json.Unmarshal([]byte(line), &fields) == nil {
		entry.Fields = fields
	}
	mu.Lock()
	entries = append(entries, entry)
	mu.Unlock()
}

// WaitFor returns the first JSON entry matching the predicate, failing the test
// when none arrives in time. Log lines cross a pipe, so they land shortly after
// the call that wrote them returns.
func WaitFor(t *testing.T, description string, matches func(Entry) bool) Entry {
	t.Helper()
	var found Entry
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, entry := range entries {
			if entry.Fields != nil && matches(entry) {
				found = entry
				return true
			}
		}
		return false
	}, waitTimeout, pollInterval, "no log entry: %s", description)
	return found
}

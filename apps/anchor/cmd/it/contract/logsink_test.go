//go:build contract

package contract_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const (
	barrierKey      = "__contract_barrier"
	maxLogLineBytes = 4 << 20
	barrierTimeout  = 5 * time.Second
)

// logSink reads the server's stdout through a pipe and indexes each JSON line
// by its request_id. Non-JSON lines go to the real stdout.
type logSink struct {
	writer  *os.File
	reader  *os.File
	forward io.Writer

	mu        sync.Mutex
	byRequest map[string][]map[string]any
	barriers  map[int64]chan struct{}
	nextID    int64
	discard   bool
	dump      io.Writer
}

func newLogSink() (*logSink, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	sink := &logSink{
		writer:    writer,
		reader:    reader,
		forward:   io.Discard,
		byRequest: map[string][]map[string]any{},
		barriers:  map[int64]chan struct{}{},
	}
	// CONTRACT_LOG_DUMP=<file> keeps every server line, for debugging.
	if path := os.Getenv("CONTRACT_LOG_DUMP"); path != "" {
		dump, createErr := os.Create(path)
		if createErr != nil {
			return nil, createErr
		}
		sink.dump = dump
	}
	go sink.run()
	return sink, nil
}

func (s *logSink) run() {
	scanner := bufio.NewScanner(s.reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLogLineBytes)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] != '{' {
			s.mu.Lock()
			forward := s.forward
			s.mu.Unlock()
			_, _ = fmt.Fprintf(forward, "%s\n", line)
			continue
		}
		s.handle(line)
	}
}

func (s *logSink) handle(line []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dump != nil && !bytes.Contains(line, []byte(barrierKey)) {
		_, _ = fmt.Fprintf(s.dump, "%s\n", line)
	}
	if s.discard && !bytes.Contains(line, []byte(barrierKey)) {
		return
	}
	var fields map[string]any
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	if err := decoder.Decode(&fields); err != nil {
		return
	}
	if raw, ok := fields[barrierKey].(json.Number); ok {
		id, _ := raw.Int64()
		if ch, found := s.barriers[id]; found {
			close(ch)
			delete(s.barriers, id)
		}
		return
	}
	requestID, _ := fields["request_id"].(string)
	if requestID == "" {
		return
	}
	s.byRequest[requestID] = append(s.byRequest[requestID], fields)
}

// barrier writes a marker through the same pipe the server logs to and waits
// until the reader has consumed it, so every line written before it is
// indexed. os.File serialises concurrent writes, so lines never interleave.
func (s *logSink) barrier() error {
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	ch := make(chan struct{})
	s.barriers[id] = ch
	s.mu.Unlock()
	if _, err := fmt.Fprintf(s.writer, "{%q:%d}\n", barrierKey, id); err != nil {
		return err
	}
	select {
	case <-ch:
		return nil
	case <-time.After(barrierTimeout):
		return fmt.Errorf("log barrier %d timed out", id)
	}
}

// linesFor returns the lines logged for requestID once the request summary
// (the line with a status key) has arrived, or after a short grace period for
// requests that never log one (no route, /health).
func (s *logSink) linesFor(requestID string) ([]map[string]any, error) {
	const (
		attempts = 5
		pause    = 20 * time.Millisecond
	)
	for attempt := range attempts {
		if err := s.barrier(); err != nil {
			return nil, err
		}
		s.mu.Lock()
		lines := append([]map[string]any(nil), s.byRequest[requestID]...)
		s.mu.Unlock()
		for _, line := range lines {
			if _, ok := line["status"]; ok {
				return lines, nil
			}
		}
		if attempt < attempts-1 {
			time.Sleep(pause)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.byRequest[requestID]...), nil
}

// setDiscard stops indexing lines (benchmarks), keeping the pipe drained.
func (s *logSink) setDiscard(discard bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discard = discard
	if discard {
		s.byRequest = map[string][]map[string]any{}
	}
}

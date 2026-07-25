package logx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
	"github.com/rs/zerolog"

	"anchor/internal/logx"
)

func TestIsContextError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"canceled", context.Canceled, true},
		{"deadline", context.DeadlineExceeded, true},
		{"wrapped canceled", fmt.Errorf("update SENT status: %w", context.Canceled), true},
		// go-jet prefixes driver errors with "jet: " and breaks the Unwrap chain,
		// so errors.Is misses these; they must still be treated as cancellations.
		{"jet-wrapped canceled (broken chain)", errors.New("jet: context canceled"), true},
		{"jet-wrapped deadline (broken chain)", errors.New("jet: context deadline exceeded"), true},
		// lib/pq raises SQLSTATE 57014 when it cancels an in-flight query.
		{"pq query canceled 57014", &pq.Error{Code: "57014", Message: "canceling statement due to user request"}, true},
		// statement_timeout shares SQLSTATE 57014 but is a real server-side fault:
		// it must keep error severity instead of being hidden as a cancellation.
		{
			"pq 57014 from statement timeout is not a cancellation",
			&pq.Error{Code: "57014", Message: "canceling statement due to statement timeout"},
			false,
		},
		{"pq other error", &pq.Error{Code: "3D000", Message: "database does not exist"}, false},
		{"plain error", errors.New("boom"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := logx.IsContextError(tc.err); got != tc.want {
				t.Fatalf("IsContextError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestEventForErrorLevel(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		wantLevel string
	}{
		{"context canceled downgraded to warn", context.Canceled, "warn"},
		{"deadline exceeded downgraded to warn", context.DeadlineExceeded, "warn"},
		{"wrapped cancellation downgraded to warn", fmt.Errorf("call: %w", context.Canceled), "warn"},
		{"jet-wrapped cancellation downgraded to warn", errors.New("jet: context canceled"), "warn"},
		{
			"pq 57014 downgraded to warn",
			&pq.Error{Code: "57014", Message: "canceling statement due to user request"},
			"warn",
		},
		{
			"pq 57014 from statement timeout stays error",
			&pq.Error{Code: "57014", Message: "canceling statement due to statement timeout"},
			"error",
		},
		{"real error stays error", errors.New("db exploded"), "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf)
			logx.EventForError(&logger, tc.err).Err(tc.err).Msg("update SENT status")

			var entry struct {
				Level string `json:"level"`
			}
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("unmarshal log line: %v (line=%q)", err, buf.String())
			}
			if entry.Level != tc.wantLevel {
				t.Fatalf("level = %q, want %q", entry.Level, tc.wantLevel)
			}
		})
	}
}

package logx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
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
		{"plain error", errors.New("boom"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsContextError(tc.err); got != tc.want {
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
		{"real error stays error", errors.New("db exploded"), "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf)
			EventForError(&logger, tc.err).Err(tc.err).Msg("update SENT status")

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

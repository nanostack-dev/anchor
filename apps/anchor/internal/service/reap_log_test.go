package service

import (
	"testing"

	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/rs/zerolog"
)

func TestReapLogLevel(t *testing.T) {
	tests := []struct {
		name   string
		result pgqueue.ReapResult
		want   zerolog.Level
	}{
		{"nothing reaped stays a warning", pgqueue.ReapResult{}, zerolog.WarnLevel},
		{"only requeued is routine recovery", pgqueue.ReapResult{Requeued: 3}, zerolog.WarnLevel},
		{"failed jobs escalate to error", pgqueue.ReapResult{Requeued: 1, Failed: 2}, zerolog.ErrorLevel},
		{"failed only escalates to error", pgqueue.ReapResult{Failed: 1}, zerolog.ErrorLevel},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := reapLogLevel(tc.result); got != tc.want {
				t.Fatalf("reapLogLevel(%+v) = %v, want %v", tc.result, got, tc.want)
			}
		})
	}
}

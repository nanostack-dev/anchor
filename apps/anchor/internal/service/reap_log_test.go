package service_test

import (
	"log/slog"
	"testing"

	"github.com/nanostack-dev/pgkit/queue"

	"anchor/internal/service"
)

func TestReapLogLevel(t *testing.T) {
	tests := []struct {
		name   string
		result queue.ReapResult
		want   slog.Level
	}{
		{"nothing reaped stays a warning", queue.ReapResult{}, slog.LevelWarn},
		{"only requeued is routine recovery", queue.ReapResult{Requeued: 3}, slog.LevelWarn},
		{"failed jobs escalate to error", queue.ReapResult{Requeued: 1, Failed: 2}, slog.LevelError},
		{"failed only escalates to error", queue.ReapResult{Failed: 1}, slog.LevelError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.ReapLogLevel(tc.result); got != tc.want {
				t.Fatalf("ReapLogLevel(%+v) = %v, want %v", tc.result, got, tc.want)
			}
		})
	}
}

package applog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/applog"
)

func newLoggers(buf *bytes.Buffer) applog.Loggers {
	return applog.New(log.Config{
		Wire:        log.WireLegacy,
		Output:      buf,
		Level:       slog.LevelDebug,
		Environment: "development",
		Service:     "anchor",
	})
}

func lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		var fields map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &fields), line)
		out = append(out, fields)
	}
	return out
}

// requestContext opens a request scope as log.HTTPMiddleware does.
func requestContext() context.Context {
	ctx := log.WithScope(context.Background())
	log.SetAttrs(ctx, slog.String(log.KeyRequestID, "req_1"), slog.String(log.KeyMethod, "GET"))
	return ctx
}

func TestLinesHaveTheZerologShape(t *testing.T) {
	var buf bytes.Buffer
	loggers := newLoggers(&buf)
	loggers.Component.InfoContext(context.Background(), "started", slog.Duration("duration", 1500*time.Microsecond))

	line := lines(t, &buf)[0]
	assert.Equal(t, "info", line["level"])
	assert.Equal(t, "started", line["message"])
	assert.InDelta(t, 1.5, line["duration"], 1e-9, "durations are float milliseconds")
	assert.Regexp(t, `^/.+/applog_test\.go:\d+$`, line["caller"])
	assert.Regexp(t, `^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(Z|[+-]\d\d:\d\d)$`, line["time"],
		"the time has whole seconds, as zerolog wrote it")
}

func TestScopedLinesCarryTheRequestScope(t *testing.T) {
	var buf bytes.Buffer
	newLoggers(&buf).Scoped.InfoContext(requestContext(), "scoped")

	line := lines(t, &buf)[0]
	assert.Equal(t, "req_1", line["request_id"])
	assert.Equal(t, "GET", line["method"])
}

// The component logger writes without the context's attributes, as the
// injected zerolog logger did: the golden suite pins the lines of a request.
func TestComponentLinesLeaveTheRequestScopeOut(t *testing.T) {
	var buf bytes.Buffer
	newLoggers(&buf).Component.InfoContext(requestContext(), "detached")

	line := lines(t, &buf)[0]
	assert.NotContains(t, line, "request_id")
	assert.NotContains(t, line, "method")
	assert.Equal(t, "development", line["env"], "a line outside a request carries the identity")
}

// A request's lines get the identity from auth (log.SetAttrs), so the lines
// of a public route have no env, as with pkg/log's log.Bind.
func TestRequestLinesGetTheIdentityOnlyFromTheScope(t *testing.T) {
	var buf bytes.Buffer
	loggers := newLoggers(&buf)
	ctx := requestContext()

	loggers.Scoped.InfoContext(ctx, "before auth")
	log.SetAttrs(ctx, loggers.Identity...)
	loggers.Scoped.InfoContext(ctx, "after auth")

	got := lines(t, &buf)
	assert.NotContains(t, got[0], "env")
	assert.NotContains(t, got[0], "service")
	assert.Equal(t, "development", got[1]["env"])
	assert.Equal(t, "anchor", got[1]["service"])
}

func TestConfigFromEnvDefaultsToTheLegacyWire(t *testing.T) {
	cfg, err := applog.ConfigFromEnv(func(string) string { return "" })
	require.NoError(t, err)
	assert.Equal(t, log.WireLegacy, cfg.Wire)

	cfg, err = applog.ConfigFromEnv(func(key string) string {
		if key == "LOG_WIRE" {
			return "slog"
		}
		return ""
	})
	require.NoError(t, err)
	assert.Equal(t, log.WireSlog, cfg.Wire)
}

func TestBurstSamplerLetsBurstLinesThroughPerPeriod(t *testing.T) {
	sampler := &applog.BurstSampler{Burst: 2, Period: time.Hour}

	assert.True(t, sampler.Sample())
	assert.True(t, sampler.Sample())
	assert.False(t, sampler.Sample())
}

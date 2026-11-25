package applog_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/lib/pq"
	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/stretchr/testify/assert"

	"anchor/internal/applog"
)

// errorLine logs err with log.ErrorTo on anchor's component logger. The tests
// pin the levels and keys that anchor's local error helper wrote before
// core/log gave the same ones (framework v0.23.0-beta.2).
func errorLine(err error) string {
	var buf bytes.Buffer
	logger := applog.New(log.Config{Wire: log.WireLegacy, Output: &buf, Level: slog.LevelDebug}).Component
	log.ErrorTo(context.Background(), logger, err, "failed to search organization invitations")
	return buf.String()
}

func TestErrorLogsAnInvalidCursorAtDebug(t *testing.T) {
	line := errorLine(fmt.Errorf("search invitations: %w: not a cursor payload", search.ErrInvalidCursor))

	assert.Contains(t, line, `"level":"debug"`)
}

func TestErrorLogsACursorWithOffsetAtDebug(t *testing.T) {
	line := errorLine(fmt.Errorf("search invitations: %w", search.ErrCursorWithOffset))

	assert.Contains(t, line, `"level":"debug"`)
}

func TestErrorLogsAnUnknownSortAtDebug(t *testing.T) {
	line := errorLine(fmt.Errorf("search invitations: %w", search.ErrUnknownSort))

	assert.Contains(t, line, `"level":"debug"`)
}

func TestErrorLogsAClientFaultAtDebugWithItsStatus(t *testing.T) {
	line := errorLine(fault.NotFound("ORGANIZATION_NOT_FOUND", "organization not found"))

	assert.Contains(t, line, `"level":"debug"`)
	assert.Contains(t, line, `"status":404`)
	assert.Contains(t, line, `"error":"organization not found"`)
}

func TestErrorLogsAServerFailureAtError(t *testing.T) {
	line := errorLine(errors.New("connection refused"))

	assert.Contains(t, line, `"level":"error"`)
	assert.Contains(t, line, `"error":"connection refused"`)
}

func TestErrorLogsACancellationAtWarn(t *testing.T) {
	assert.Contains(t, errorLine(fmt.Errorf("load: %w", context.Canceled)), `"level":"warn"`)
}

func TestIsContextErrorMatchesTheDriverCancelledStatement(t *testing.T) {
	canceled := &pq.Error{Code: "57014", Message: "canceling statement due to user request"}

	assert.True(t, log.IsContextError(fmt.Errorf("query: %w", canceled)))
	assert.True(t, log.IsContextError(context.DeadlineExceeded))
	assert.False(t, log.IsContextError(errors.New("connection refused")))
}

func TestErrorLogsTheDriverCancelledStatementAtWarn(t *testing.T) {
	canceled := &pq.Error{Code: "57014", Message: "canceling statement due to user request"}

	assert.Contains(t, errorLine(fmt.Errorf("query: %w", canceled)), `"level":"warn"`)
}

func TestErrorLogsAStatementTimeoutAtError(t *testing.T) {
	timeout := &pq.Error{Code: "57014", Message: "canceling statement due to statement timeout"}

	assert.Contains(t, errorLine(fmt.Errorf("query: %w", timeout)), `"level":"error"`)
}

func TestErrorLogsAServerFaultAtErrorWithItsStatus(t *testing.T) {
	line := errorLine(fault.Internal("EMAIL_DELIVERY_FAILED", "delivery failed"))

	assert.Contains(t, line, `"level":"error"`)
	assert.Contains(t, line, `"status":500`)
}

func TestErrorNamesTheCallSiteAsCaller(t *testing.T) {
	assert.Contains(t, errorLine(errors.New("boom")), `level_test.go:`)
}

// rawErrorLine logs err the way a raw call site does, logger.Error with
// log.Err, on the logger that New builds.
func rawErrorLine(scoped bool, err error) string {
	var buf bytes.Buffer
	loggers := applog.New(log.Config{Wire: log.WireLegacy, Output: &buf, Level: slog.LevelDebug})
	logger := loggers.Component
	if scoped {
		logger = loggers.Scoped.Logger
	}
	logger.ErrorContext(context.Background(), "failed to search members", log.Err(err))
	return buf.String()
}

func TestRawErrorLineLogsACancellationAtWarn(t *testing.T) {
	err := fmt.Errorf("query.Count: jet: %w", context.Canceled)

	assert.Contains(t, rawErrorLine(false, err), `"level":"warn"`)
	assert.Contains(t, rawErrorLine(true, err), `"level":"warn"`)
}

func TestRawErrorLineLogsADeadlineAndADriverCancelAtWarn(t *testing.T) {
	canceled := &pq.Error{Code: "57014", Message: "canceling statement due to user request"}

	assert.Contains(t, rawErrorLine(false, context.DeadlineExceeded), `"level":"warn"`)
	assert.Contains(t, rawErrorLine(false, fmt.Errorf("query: %w", canceled)), `"level":"warn"`)
}

func TestRawErrorLineKeepsARealFailureAtError(t *testing.T) {
	timeout := &pq.Error{Code: "57014", Message: "canceling statement due to statement timeout"}

	assert.Contains(t, rawErrorLine(false, errors.New("connection refused")), `"level":"error"`)
	assert.Contains(t, rawErrorLine(false, fmt.Errorf("query: %w", timeout)), `"level":"error"`)
}

func TestRawErrorLineReadsACancellationFromWithAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := applog.New(log.Config{Wire: log.WireLegacy, Output: &buf, Level: slog.LevelDebug}).Component

	logger.With(log.Err(context.Canceled)).Error("role assignment operation failed")

	assert.Contains(t, buf.String(), `"level":"warn"`)
}

func TestRawErrorLineLeavesAnotherKeyAlone(t *testing.T) {
	var buf bytes.Buffer
	logger := applog.New(log.Config{Wire: log.WireLegacy, Output: &buf, Level: slog.LevelDebug}).Component

	logger.Error("sweep failed", slog.Any("cause", context.Canceled))

	assert.Contains(t, buf.String(), `"level":"error"`)
}

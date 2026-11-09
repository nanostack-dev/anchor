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

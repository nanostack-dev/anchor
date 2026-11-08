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
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"

	"anchor/internal/applog"
)

func errorLine(err error) string {
	var buf bytes.Buffer
	logger := applog.New(log.Config{Wire: log.WireLegacy, Output: &buf, Level: slog.LevelDebug}).Component
	applog.Error(context.Background(), logger, err, "failed to search organization invitations")
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

	assert.True(t, applog.IsContextError(fmt.Errorf("query: %w", canceled)))
	assert.True(t, applog.IsContextError(context.DeadlineExceeded))
	assert.False(t, applog.IsContextError(errors.New("connection refused")))
}

func TestErrorNamesTheCallSiteAsCaller(t *testing.T) {
	assert.Contains(t, errorLine(errors.New("boom")), `level_test.go:`)
}

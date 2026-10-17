package api_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"anchor/internal/api"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func logAPIErrorLine(err error) string {
	var buf bytes.Buffer
	api.LogAPIErrorForTest(zerolog.New(&buf), err).Msg("failed to search organization invitations")
	return buf.String()
}

func TestLogAPIErrorLogsAnInvalidCursorAtDebug(t *testing.T) {
	line := logAPIErrorLine(fmt.Errorf("search invitations: %w: not a cursor payload", search.ErrInvalidCursor))

	assert.Contains(t, line, `"level":"debug"`)
}

func TestLogAPIErrorLogsACursorWithOffsetAtDebug(t *testing.T) {
	line := logAPIErrorLine(fmt.Errorf("search invitations: %w", search.ErrCursorWithOffset))

	assert.Contains(t, line, `"level":"debug"`)
}

func TestLogAPIErrorLogsAnUnknownSortAtDebug(t *testing.T) {
	line := logAPIErrorLine(fmt.Errorf("search invitations: %w", search.ErrUnknownSort))

	assert.Contains(t, line, `"level":"debug"`)
}

func TestLogAPIErrorLogsAClientFaultAtDebugWithItsStatus(t *testing.T) {
	line := logAPIErrorLine(fault.NotFound("ORGANIZATION_NOT_FOUND", "organization not found"))

	assert.Contains(t, line, `"level":"debug"`)
	assert.Contains(t, line, `"status":404`)
}

func TestLogAPIErrorLogsAServerFailureAtError(t *testing.T) {
	line := logAPIErrorLine(errors.New("connection refused"))

	assert.Contains(t, line, `"level":"error"`)
}

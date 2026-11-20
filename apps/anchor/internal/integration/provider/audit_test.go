package provider_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/integration"
	"anchor/internal/integration/provider"
)

func TestFailureAudit(t *testing.T) {
	t.Parallel()

	t.Run("The audit row survives wrapping and the cause stays reachable", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("duplicate key")
		row := integration.AuditLog{Action: integration.AuditActionUpsertUser, Severity: integration.AuditSeverityError}
		err := fmt.Errorf("execute commands: %w", provider.WithFailureAudit(cause, row))

		got, ok := provider.FailureAudit(err)
		require.True(t, ok)
		assert.Equal(t, row, got)
		require.ErrorIs(t, err, cause)
		assert.Equal(t, "execute commands: duplicate key", err.Error())
	})

	t.Run("An error without an audit row reports none", func(t *testing.T) {
		t.Parallel()

		_, ok := provider.FailureAudit(errors.New("plain"))
		assert.False(t, ok)
	})
}

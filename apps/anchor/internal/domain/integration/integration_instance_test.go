package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/integration"
)

func TestInstanceIngestionDecision(t *testing.T) {
	t.Parallel()
	secret := "webhook-secret"
	blank := "  "
	diagnostic := "active"
	empty := ""
	for _, test := range []struct {
		name      string
		enabled   bool
		secret    *string
		status    integration.Status
		lastError *string
		canIngest bool
		reason    string
	}{
		{"disabled precedes missing secret and error", false, nil, integration.StatusError, &diagnostic, false, "disabled"},
		{"missing secret precedes active", true, nil, integration.StatusActive, nil, false, "missing_webhook_secret"},
		{"blank secret precedes error", true, &blank, integration.StatusError, &diagnostic, false, "missing_webhook_secret"},
		{"active", true, &secret, integration.StatusActive, nil, true, "active"},
		{"configuring", true, &secret, integration.StatusConfiguring, nil, false, "configuring"},
		{"inactive", true, &secret, integration.StatusInactive, nil, false, "inactive"},
		{"error without detail", true, &secret, integration.StatusError, nil, false, "error"},
		{"error with empty detail", true, &secret, integration.StatusError, &empty, false, "error"},
		{"diagnostic text cannot authorize ingestion", true, &secret, integration.StatusError, &diagnostic, false, "active"},
		{"unknown state remains blocked", true, &secret, integration.Status("UNKNOWN"), nil, false, "unknown_state"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			instance := integration.Instance{
				IsEnabled:     test.enabled,
				WebhookSecret: test.secret,
				Status:        test.status,
				LastError:     test.lastError,
			}
			assert.Equal(t, test.canIngest, instance.CanIngest())
			assert.Equal(t, test.reason, instance.IngestionBlockReason())
		})
	}
}

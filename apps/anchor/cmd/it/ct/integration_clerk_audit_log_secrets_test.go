package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// Not parallel: its keyed Clerk instance seeds the process-wide reconcile scheduler.
func TestIntegrationAuditLogs_DoNotLeakSecrets(t *testing.T) {
	productContext := createTestProductContext(t)
	instance := createClerkIntegrationInstance(t, productContext)
	t.Cleanup(func() {
		resp, err := productContext.OwnerAuthenticatedClient().DeleteIntegrationInstanceWithResponse(
			context.Background(), productContext.ProductID, instance.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode())
	})

	webhookSecret := "whsec_super_secret_for_audit_test"
	apiKey := "sk_test_super_secret_for_audit_test"

	cfg := ct.IntegrationProviderConfig{}
	require.NoError(t, cfg.FromClerkIntegrationConfig(ct.ClerkIntegrationConfig{
		ApiKey: new(apiKey),
	}))

	_ = updateClerkIntegrationInstance(
		t,
		productContext,
		instance.Id,
		ct.UpdateIntegrationInstanceJSONRequestBody{
			WebhookSecret: new(webhookSecret),
			Config:        &cfg,
		},
	)

	auditLogs := listIntegrationAuditLogs(t, productContext, instance.Id)
	require.NotEmpty(t, auditLogs.Items)

	for _, item := range auditLogs.Items {
		raw, err := json.Marshal(item)
		require.NoError(t, err)
		serialized := strings.ToLower(string(raw))

		require.NotContains(t, serialized, strings.ToLower(webhookSecret),
			"audit log entry must not leak webhook secret")
		require.NotContains(t, serialized, strings.ToLower(apiKey),
			"audit log entry must not leak clerk api key")
	}
}

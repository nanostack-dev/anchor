package ct_test

import (
	"encoding/json"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
	"github.com/stretchr/testify/require"
)

func TestIntegrationAuditLogs_DoNotLeakSecrets(t *testing.T) {
	productContext := createTestProductContext(t)
	instance := createClerkIntegrationInstance(t, productContext)

	webhookSecret := "whsec_super_secret_for_audit_test"
	apiKey := "sk_test_super_secret_for_audit_test"

	cfg := ct.IntegrationProviderConfig{}
	require.NoError(t, cfg.FromClerkIntegrationConfig(ct.ClerkIntegrationConfig{
		ApiKey: ptr.Ptr(apiKey),
	}))

	_ = updateClerkIntegrationInstance(
		t,
		productContext,
		instance.Id,
		ct.UpdateIntegrationInstanceJSONRequestBody{
			WebhookSecret: ptr.Ptr(webhookSecret),
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

package webhooksecret_ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/cmd/it/shared/mailpit"
)

// TestSMTPActiveInstanceUpdateDoesNotRequireWebhookSecret is the regression
// guard for the bug where updating an active SMTP integration returned
// "Webhook secret is required when integration instance is active" even though
// SMTP is outbound-only and never ingests webhooks.
func TestSMTPActiveInstanceUpdateDoesNotRequireWebhookSecret(t *testing.T) {
	mp := mailpit.Start(t)
	defer mp.Stop(t)

	tc := newTestCtx(t)
	instance := seedActiveSMTPInstance(t, tc, mp)

	// A plain config update on the active instance — no webhook secret in sight.
	cfg := ct.IntegrationProviderConfig{}
	require.NoError(t, cfg.FromSmtpIntegrationConfig(ct.SmtpIntegrationConfig{
		Host:        ptr.Ptr(mp.SMTPHost),
		Port:        ptr.Ptr(mp.SMTPPort),
		Encryption:  ptr.Ptr(ct.SmtpIntegrationConfigEncryptionNONE),
		AuthMethod:  ptr.Ptr(ct.SmtpIntegrationConfigAuthMethodPLAIN),
		Username:    ptr.Ptr("test"),
		FromAddress: ptr.Ptr("noreply@tryanchor.dev"),
		FromName:    ptr.Ptr("Anchor Updated"),
	}))

	resp := updateInstance(t, tc, instance.ID, ct.UpdateIntegrationInstanceJSONRequestBody{
		Config: &cfg,
	})

	require.Equal(t, http.StatusOK, resp.StatusCode(), "body: %s", string(resp.Body))
	require.NotNil(t, resp.JSON200)
	assert.Equal(t, ct.IntegrationInstanceStatusACTIVE, resp.JSON200.Status)
}

// TestClerkActiveInstanceStillRequiresWebhookSecret proves the gate is not
// disabled wholesale: a webhook-ingesting provider (CLERK) that loses its
// secret while active is still rejected.
func TestClerkActiveInstanceStillRequiresWebhookSecret(t *testing.T) {
	tc := newTestCtx(t)
	instance := seedActiveClerkInstance(t, tc)

	resp := updateInstance(t, tc, instance.ID, ct.UpdateIntegrationInstanceJSONRequestBody{
		WebhookSecret: ptr.Ptr(""),
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "body: %s", string(resp.Body))
	require.NotNil(t, resp.JSON400)
	require.Len(t, resp.JSON400.Errors, 1)
	assert.Equal(t, "INTEGRATION_WEBHOOK_SECRET_REQUIRED", resp.JSON400.Errors[0].Code)
}

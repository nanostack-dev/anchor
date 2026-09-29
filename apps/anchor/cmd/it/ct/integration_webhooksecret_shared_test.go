package ct_test

import (
	"context"
	"encoding/json"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/cmd/it/shared/mailpit"
	domainintegration "anchor/internal/domain/integration"
	smtpprov "anchor/internal/integration/provider/smtp"
)

// Webhook-secret activation gate: updates flow through the real chi router and
// oapi-codegen strict server, backed by a live Postgres; an active SMTP instance
// points at the shared mailpit container so the outbound provider has a
// reachable, verifiable endpoint.
//
// Regression coverage: an outbound-only provider (SMTP) must NOT require a
// webhook secret to stay active, while a webhook-ingesting provider (CLERK)
// must. See validateInstanceIngestionState in internal/service.
const webhookSecretClerkSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"

type webhookSecretTestCtx struct {
	tenantID string
	product  *itdsl.ProductContext
}

func newWebhookSecretTestCtx(t *testing.T) webhookSecretTestCtx {
	t.Helper()
	tenantAlias := "tenant." + itshared.Faker.UUID().V4()
	productAlias := "product." + itshared.Faker.UUID().V4()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: tenantAlias, Isolated: true}).
		Product(itdsl.ProductOpts{Alias: productAlias, TenantAlias: tenantAlias}).
		Build()
	return webhookSecretTestCtx{
		tenantID: state.Tenant(tenantAlias).ID,
		product:  state.Product(productAlias),
	}
}

// seedActiveSMTPInstance creates an ACTIVE outbound SMTP instance wired to the
// given mailpit container, bypassing the async verify-and-activate dance.
func seedActiveSMTPInstance(t *testing.T, tc webhookSecretTestCtx, mp *mailpit.Mailpit) domainintegration.Instance {
	t.Helper()
	cfg := smtpprov.Config{
		Host:        mp.SMTPHost,
		Port:        mp.SMTPPort,
		Encryption:  smtpprov.EncryptionNone,
		AuthMethod:  smtpprov.AuthMethodPlain,
		Username:    "test",
		Password:    "test",
		FromAddress: "noreply@tryanchor.dev",
		FromName:    "Anchor",
	}
	cfgJSON, err := json.Marshal(cfg)
	require.NoError(t, err)

	inst := domainintegration.Instance{
		PlatformTenantID: tc.tenantID,
		ProductID:        tc.product.ProductID,
		ProviderType:     domainintegration.ProviderTypeSMTP,
		ConfigJSON:       cfgJSON,
		ConfigVersion:    1,
		IsEnabled:        true,
		Status:           domainintegration.StatusActive,
	}
	inst.GenerateID()

	created, err := IntegrationRepo.Create(context.Background(), inst)
	require.NoError(t, err)
	return created
}

// seedActiveClerkInstance creates an ACTIVE webhook-ingesting CLERK instance
// that already carries a webhook secret.
func seedActiveClerkInstance(t *testing.T, tc webhookSecretTestCtx) domainintegration.Instance {
	t.Helper()
	secret := webhookSecretClerkSecret
	inst := domainintegration.Instance{
		PlatformTenantID: tc.tenantID,
		ProductID:        tc.product.ProductID,
		ProviderType:     domainintegration.ProviderTypeClerk,
		WebhookSecret:    &secret,
		ConfigJSON:       json.RawMessage(`{}`),
		ConfigVersion:    1,
		IsEnabled:        true,
		Status:           domainintegration.StatusActive,
	}
	inst.GenerateID()

	created, err := IntegrationRepo.Create(context.Background(), inst)
	require.NoError(t, err)
	return created
}

func updateInstance(
	t *testing.T,
	tc webhookSecretTestCtx,
	instanceID string,
	body ct.UpdateIntegrationInstanceJSONRequestBody,
) *ct.UpdateIntegrationInstanceResponse {
	t.Helper()
	resp, err := tc.product.OwnerAuthenticatedClient().UpdateIntegrationInstanceWithResponse(
		context.Background(),
		tc.product.ProductID,
		instanceID,
		body,
	)
	require.NoError(t, err)
	return resp
}

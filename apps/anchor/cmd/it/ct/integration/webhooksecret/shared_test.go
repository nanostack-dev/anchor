// Package webhooksecret_ct_test contains component tests for the webhook-secret
// activation gate on integration instances. Updates flow through the real chi
// router and oapi-codegen strict server, backed by a live Postgres; an active
// SMTP instance points at a mailpit SMTP container so the outbound provider has
// a reachable, verifiable endpoint.
//
// Regression coverage: an outbound-only provider (SMTP) must NOT require a
// webhook secret to stay active, while a webhook-ingesting provider (CLERK)
// must. See validateInstanceIngestionState in internal/service.
package webhooksecret_ct_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/cmd/it/shared/mailpit"
	domainintegration "anchor/internal/domain/integration"
	smtpprov "anchor/internal/integration/provider/smtp"
	intrepo "anchor/internal/repository"
)

const clerkTestWebhookSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"

var IntegrationRepo intrepo.IntegrationInstanceRepository

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	itshared.RunTestMain(m, itshared.TestConfig{
		EnableRedis:             true,
		PopulateRepositories:    true,
		APIKeyService:           &itshared.APIKeyService,
		PermissionRepository:    &itshared.PermissionRepository,
		ProductRepository:       &itshared.ProductRepository,
		ProductUserRepository:   &itshared.ProductUserRepository,
		OrgMembershipRepository: &itshared.OrgMembershipRepository,
		TenantRepository:        &itshared.TenantRepository,
		UserRepository:          &itshared.UserRepository,
		PlatformUserRepository:  &itshared.PlatformTenantUserRepo,
		JWTHelper:               &itshared.JWTHelper,
		ExtraPopulateTargets:    []interface{}{&IntegrationRepo},
	})
}

type testCtx struct {
	tenantID string
	product  *itdsl.ProductContext
}

func newTestCtx(t *testing.T) testCtx {
	t.Helper()
	tenantAlias := "tenant." + itshared.Faker.UUID().V4()
	productAlias := "product." + itshared.Faker.UUID().V4()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: tenantAlias, Isolated: true}).
		Product(itdsl.ProductOpts{Alias: productAlias, TenantAlias: tenantAlias}).
		Build()
	return testCtx{
		tenantID: state.Tenant(tenantAlias).ID,
		product:  state.Product(productAlias),
	}
}

// seedActiveSMTPInstance creates an ACTIVE outbound SMTP instance wired to the
// given mailpit container, bypassing the async verify-and-activate dance.
func seedActiveSMTPInstance(t *testing.T, tc testCtx, mp *mailpit.Mailpit) domainintegration.Instance {
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
func seedActiveClerkInstance(t *testing.T, tc testCtx) domainintegration.Instance {
	t.Helper()
	secret := clerkTestWebhookSecret
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
	tc testCtx,
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

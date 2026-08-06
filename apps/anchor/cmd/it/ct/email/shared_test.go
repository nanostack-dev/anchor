// Package email_ct_test contains component tests for the email HTTP API layer.
// Every test path goes through the real chi router and oapi-codegen strict
// server, backed by a live Postgres. Delivery tests use a mailpit SMTP container.
// Fixtures are seeded directly via repositories; no service is injected.
package email_ct_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/cmd/it/shared/mailpit"
	domainintegration "anchor/internal/domain/integration"
	smtpprov "anchor/internal/integration/provider/smtp"
	intrepo "anchor/internal/repository"
)

var IntegrationRepo intrepo.IntegrationInstanceRepository

func TestMain(m *testing.M) {
	if err := os.Chdir(".."); err != nil {
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
		ExtraPopulateTargets:    []any{&IntegrationRepo},
		AfterRun:                mailpit.StopShared,
	})
}

type testCtx struct {
	tenantID string
	product  *itdsl.ProductContext
}

func newTestCtx(t *testing.T) testCtx {
	t.Helper()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true}).
		Product(itdsl.ProductOpts{Alias: "p", TenantAlias: "t"}).
		Build()
	return testCtx{
		tenantID: state.Tenant("t").ID,
		product:  state.Product("p"),
	}
}

func seedSMTPInstance(t *testing.T, tc testCtx, mp *mailpit.Mailpit) {
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
	_, err = IntegrationRepo.Create(context.Background(), inst)
	require.NoError(t, err)
}

func uniqueSlug() string {
	return "tpl-" + ids.MustNew("ct")
}

func assertAPIError(t *testing.T, errs []ct.ApiError, code, message string) {
	t.Helper()
	assert.Len(t, errs, 1)
	if len(errs) > 0 {
		assert.Equal(t, code, errs[0].Code)
		assert.Equal(t, message, errs[0].Message)
	}
}

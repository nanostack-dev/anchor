package ct_test

import (
	"context"
	"encoding/json"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/cmd/it/shared/mailpit"
	domainintegration "anchor/internal/domain/integration"
	smtpprov "anchor/internal/integration/provider/smtp"
)

// Email component tests go through the real chi router and oapi-codegen strict
// server, backed by a live Postgres. Delivery tests use the shared mailpit SMTP
// container. Fixtures are seeded directly via repositories; no service is injected.
type emailTestCtx struct {
	tenantID string
	product  *itdsl.ProductContext
}

func newEmailTestCtx(t *testing.T) emailTestCtx {
	t.Helper()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true}).
		Product(itdsl.ProductOpts{Alias: "p", TenantAlias: "t"}).
		Build()
	return emailTestCtx{
		tenantID: state.Tenant("t").ID,
		product:  state.Product("p"),
	}
}

func seedSMTPInstance(t *testing.T, tc emailTestCtx, mp *mailpit.Mailpit) {
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

func assertEmailAPIError(t *testing.T, errs []ct.ApiError, code, message string) {
	t.Helper()
	assert.Len(t, errs, 1)
	if len(errs) > 0 {
		assert.Equal(t, code, errs[0].Code)
		assert.Equal(t, message, errs[0].Message)
	}
}

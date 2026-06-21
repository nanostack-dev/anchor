package email_ct_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainintegration "anchor/internal/domain/integration"
	smtpprov "anchor/internal/integration/provider/smtp"
)

// TestEmailSendDeliveryFailure verifies that when the configured SMTP
// integration cannot deliver (here: the relay refuses the connection), the API
// returns a modelled EMAIL_DELIVERY_FAILED error rather than a generic
// "unhandled error" 500. Delivery failure is a known operational outcome, so it
// must carry a stable, machine-readable code for callers and dashboards.
func TestEmailSendDeliveryFailure(t *testing.T) {
	tc := newTestCtx(t)
	seedUnreachableSMTPInstance(t, tc)
	client := tc.product.OwnerAuthenticatedClient()

	created, err := client.CreateEmailTemplateWithResponse(
		context.Background(),
		tc.product.ProductID,
		ct.CreateEmailTemplateJSONRequestBody{
			Slug:     uniqueSlug(),
			Name:     "Transactional",
			Subject:  "Hello {{ .name }}",
			BodyHtml: "<p>Hi {{ .name }}</p>",
		},
	)
	require.NoError(t, err)
	tplID := created.JSON201.Id

	_, err = client.PublishEmailTemplateWithResponse(context.Background(), tc.product.ProductID, tplID)
	require.NoError(t, err)

	vars := map[string]any{"name": "Bob"}
	resp, sendErr := client.SendEmailWithResponse(
		context.Background(),
		tc.product.ProductID,
		ct.SendEmailJSONRequestBody{
			TemplateId: &tplID,
			ToAddress:  openapi_types.Email("bob@example.com"),
			Variables:  &vars,
		},
	)
	require.NoError(t, sendErr)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode())

	// The generated client does not model a 500 body, so decode the standard
	// error envelope directly.
	var env ct.ApiErrorResponse
	require.NoError(t, json.Unmarshal(resp.Body, &env))
	require.Len(t, env.Errors, 1)
	assert.Equal(t, "EMAIL_DELIVERY_FAILED", env.Errors[0].Code)
	// The underlying transport cause must never leak to the client.
	assert.NotContains(t, env.Errors[0].Message, "connection refused")
	assert.NotContains(t, env.Errors[0].Message, "dial")
}

// seedUnreachableSMTPInstance configures the product's SMTP integration to point
// at a closed local port so that delivery fails fast with a refused connection.
func seedUnreachableSMTPInstance(t *testing.T, tc testCtx) {
	t.Helper()

	// Reserve then release a port to guarantee nothing is listening on it.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().(*net.TCPAddr)
	host := addr.IP.String()
	port := addr.Port
	require.NoError(t, l.Close())

	cfg := smtpprov.Config{
		Host:        host,
		Port:        port,
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

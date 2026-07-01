package email_ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainintegration "anchor/internal/domain/integration"
	smtpprov "anchor/internal/integration/provider/smtp"
)

// TestEmailSendSenderIdentityFailure verifies that an integration failure that
// surfaces before dispatch — here, an SMTP config whose stored password is an
// undecryptable versioned secret, so SenderIdentity fails while decrypting it —
// is modelled as EMAIL_DELIVERY_FAILED rather than escaping to the strict
// boundary as a generic "unhandled error" 500.
//
// SenderIdentity runs ahead of the relay dial, so without explicit handling its
// error never reaches the dispatch-error classifier; this guards that the
// pre-dispatch path is classified the same way.
func TestEmailSendSenderIdentityFailure(t *testing.T) {
	tc := newTestCtx(t)
	seedUndecryptableSMTPInstance(t, tc)
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
	// error envelope directly. The failure must carry the stable delivery code,
	// not the generic UNEXPECTED produced by the unhandled-error safety net.
	var env ct.ApiErrorResponse
	require.NoError(t, json.Unmarshal(resp.Body, &env))
	require.Len(t, env.Errors, 1)
	assert.Equal(t, "EMAIL_DELIVERY_FAILED", env.Errors[0].Code)
	// The underlying decryption detail must never leak to the client.
	assert.NotContains(t, env.Errors[0].Message, "decrypt")
	assert.NotContains(t, env.Errors[0].Message, "cipher")
}

// seedUndecryptableSMTPInstance configures the product's SMTP integration with a
// password in the versioned-encrypted-secret format that cannot be decrypted
// (unknown key version / malformed payload), so resolveConfig fails inside
// SenderIdentity before any relay connection is attempted.
func seedUndecryptableSMTPInstance(t *testing.T, tc testCtx) {
	t.Helper()

	cfg := smtpprov.Config{
		Host:        "smtp.example.com",
		Port:        587,
		Encryption:  smtpprov.EncryptionStartTLS,
		AuthMethod:  smtpprov.AuthMethodPlain,
		Username:    "test",
		Password:    "enc:v1:AAAA", // versioned-secret shape, but undecryptable
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

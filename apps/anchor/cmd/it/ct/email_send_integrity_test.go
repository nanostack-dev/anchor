package ct_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/cmd/it/shared/mailpit"
	domainintegration "anchor/internal/domain/integration"
	smtpprov "anchor/internal/integration/provider/smtp"
)

// Not parallel: these tests reset and read the shared mailpit inbox.

func TestSendEmailKeepsALineBreakOutOfTheHeaders(t *testing.T) {
	t.Run("A line break in a subject variable adds no header to the delivered message", func(t *testing.T) {
		mp := mailpit.Shared(t)
		tc := newEmailTestCtx(t)
		seedSMTPInstance(t, tc, mp)
		owner := tc.product.OwnerAuthenticatedClient()
		tplID := publishTemplate(t, owner, tc, "Hello {{ .name }}", "<p>Hi {{ .name }}</p>")
		sendOnly, _ := tc.product.CreateAPIKeyClientWithScopes([]string{"email:send"})

		vars := map[string]any{"name": "Bob\r\nX-Injected: yes\r\nReply-To: attacker@example.test"}
		sent, err := sendOnly.SendEmailWithResponse(context.Background(), tc.product.ProductID,
			ct.SendEmailJSONRequestBody{
				TemplateId: &tplID,
				ToAddress:  openapi_types.Email(strings.ToLower(ids.MustNew("rcpt")) + "@example.com"),
				Variables:  &vars,
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, sent.StatusCode(), string(sent.Body))

		delivered := mp.WaitForMessage(t, 10*time.Second)
		var headers map[string][]string
		require.NoError(t, json.Unmarshal(
			fetchMailpit(t, mp.APIBase+"/api/v1/message/"+delivered.ID+"/headers"), &headers))
		assert.NotContains(t, headers, "X-Injected")
		assert.NotContains(t, headers, "Reply-To")
	})
}

func TestSendEmailRefusesADraftForAProductKey(t *testing.T) {
	t.Run("An email:send product key cannot send the unpublished draft", func(t *testing.T) {
		mp := mailpit.Shared(t)
		mp.Reset(t)
		tc := newEmailTestCtx(t)
		seedSMTPInstance(t, tc, mp)
		owner := tc.product.OwnerAuthenticatedClient()
		tplID := publishTemplate(t, owner, tc, "Published", "<p>Published</p>")
		draft, draftHTML := "Draft secret", "<p>Draft secret</p>"
		drafted, err := owner.UpdateEmailTemplateDraftWithResponse(context.Background(), tc.product.ProductID, tplID,
			ct.UpdateEmailTemplateDraftJSONRequestBody{Subject: &draft, BodyHtml: &draftHTML})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, drafted.StatusCode())
		sendOnly, _ := tc.product.CreateAPIKeyClientWithScopes([]string{"email:send"})

		useDraft := true
		sent, err := sendOnly.SendEmailWithResponse(context.Background(), tc.product.ProductID,
			ct.SendEmailJSONRequestBody{
				TemplateId: &tplID,
				ToAddress:  openapi_types.Email(strings.ToLower(ids.MustNew("rcpt")) + "@example.com"),
				UseDraft:   &useDraft,
			})
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, sent.StatusCode())
		assert.Empty(t, mp.Messages(t))
	})
}

func TestSendEmailRetryOfAFailedSendKeepsItsMessage(t *testing.T) {
	t.Run("A failed send's dedupe key cannot deliver another recipient or content", func(t *testing.T) {
		mp := mailpit.Shared(t)
		mp.Reset(t)
		tc := newEmailTestCtx(t)
		owner := tc.product.OwnerAuthenticatedClient()
		keyClient, _ := tc.product.CreateAPIKeyClientWithScopes([]string{"email:send"})
		instance := unreachableSMTPInstance(t, tc)
		tplID := createPublishedTemplate(t, owner, tc)
		dedupeKey := ids.MustNew("dkey")
		send := func(name string) *ct.SendEmailResponse {
			vars := map[string]any{"name": name}
			sent, err := keyClient.SendEmailWithResponse(context.Background(), tc.product.ProductID,
				ct.SendEmailJSONRequestBody{
					TemplateId: &tplID,
					ToAddress:  openapi_types.Email(strings.ToLower(name) + "-" + dedupeKey + "@example.test"),
					Variables:  &vars,
					DedupeKey:  &dedupeKey,
				})
			require.NoError(t, err)
			return sent
		}
		require.NotEqual(t, http.StatusCreated, send("Bob").StatusCode(), "the first send fails at SMTP")
		pointSMTPAt(t, tc, instance, mp)

		retried := send("Carol")

		assert.Equal(t, http.StatusConflict, retried.StatusCode(), string(retried.Body))
		assert.Empty(t, mp.Messages(t))
	})
}

func publishTemplate(t *testing.T, owner *ct.ClientWithResponses, tc emailTestCtx, subject, html string) string {
	t.Helper()
	created, err := owner.CreateEmailTemplateWithResponse(context.Background(), tc.product.ProductID,
		ct.CreateEmailTemplateJSONRequestBody{Slug: uniqueSlug(), Name: "Template", Subject: subject, BodyHtml: html})
	require.NoError(t, err)
	require.NotNil(t, created.JSON201)
	published, err := owner.PublishEmailTemplateWithResponse(context.Background(), tc.product.ProductID,
		created.JSON201.Id)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, published.StatusCode())
	return created.JSON201.Id
}

func smtpConfigJSON(t *testing.T, host string, port int) []byte {
	t.Helper()
	raw, err := json.Marshal(smtpprov.Config{
		Host: host, Port: port, Encryption: smtpprov.EncryptionNone, AuthMethod: smtpprov.AuthMethodPlain,
		Username: "test", Password: "test", FromAddress: "noreply@tryanchor.dev", FromName: "Anchor",
	})
	require.NoError(t, err)
	return raw
}

func unreachableSMTPInstance(t *testing.T, tc emailTestCtx) domainintegration.Instance {
	t.Helper()
	instance := domainintegration.Instance{
		PlatformTenantID: tc.tenantID, ProductID: tc.product.ProductID,
		ProviderType: domainintegration.ProviderTypeSMTP, ConfigJSON: smtpConfigJSON(t, "127.0.0.1", 1),
		ConfigVersion: 1, IsEnabled: true, Status: domainintegration.StatusActive,
	}
	instance.GenerateID()
	_, err := IntegrationRepo.Create(context.Background(), instance)
	require.NoError(t, err)
	return instance
}

func pointSMTPAt(t *testing.T, tc emailTestCtx, instance domainintegration.Instance, mp *mailpit.Mailpit) {
	t.Helper()
	instance.ConfigJSON = smtpConfigJSON(t, mp.SMTPHost, mp.SMTPPort)
	instance.ConfigVersion = 2
	_, err := IntegrationRepo.Update(context.Background(), tc.tenantID, instance)
	require.NoError(t, err)
}

func fetchMailpit(t *testing.T, url string) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return raw
}

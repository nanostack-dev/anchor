package email_ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/cmd/it/shared/mailpit"
	domainintegration "anchor/internal/domain/integration"
	smtpprov "anchor/internal/integration/provider/smtp"
)

// TestEmailSendRedispatchesFailedDedupe verifies that a send whose prior attempt
// for the same dedupe key FAILED is re-dispatched (not short-circuited as a stale
// failure). Otherwise a transient SMTP blip would permanently suppress the email
// while reporting success.
func TestEmailSendRedispatchesFailedDedupe(t *testing.T) {
	mp := mailpit.Start(t)
	defer mp.Stop(t)

	tc := newTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()

	// Start with an unreachable SMTP so the first send FAILS and leaves a FAILED
	// audit row carrying the dedupe key.
	badCfg, err := json.Marshal(smtpprov.Config{
		Host:        "127.0.0.1",
		Port:        1,
		Encryption:  smtpprov.EncryptionNone,
		AuthMethod:  smtpprov.AuthMethodPlain,
		Username:    "test",
		Password:    "test",
		FromAddress: "noreply@tryanchor.dev",
		FromName:    "Anchor",
	})
	require.NoError(t, err)
	inst := domainintegration.Instance{
		PlatformTenantID: tc.tenantID,
		ProductID:        tc.product.ProductID,
		ProviderType:     domainintegration.ProviderTypeSMTP,
		ConfigJSON:       badCfg,
		ConfigVersion:    1,
		IsEnabled:        true,
		Status:           domainintegration.StatusActive,
	}
	inst.GenerateID()
	_, err = IntegrationRepo.Create(context.Background(), inst)
	require.NoError(t, err)

	tplID := createPublishedTemplate(t, client, tc)

	dedupeKey := ids.MustNew("dkey")
	vars := map[string]interface{}{"name": "Bob"}
	body := ct.SendEmailJSONRequestBody{
		TemplateId: &tplID,
		ToAddress:  openapi_types.Email("bob@example.com"),
		ToName:     ptr.Ptr("Bob"),
		Variables:  &vars,
		DedupeKey:  &dedupeKey,
	}

	// First send fails to dispatch (SMTP unreachable) → FAILED record, not 201.
	first, err := client.SendEmailWithResponse(context.Background(), tc.product.ProductID, body)
	require.NoError(t, err)
	require.NotEqual(t, http.StatusCreated, first.StatusCode(), "first send must fail at SMTP")
	require.Empty(t, mp.Messages(t), "nothing delivered while SMTP was unreachable")

	// Repair SMTP to point at mailpit.
	goodCfg, err := json.Marshal(smtpprov.Config{
		Host:        mp.SMTPHost,
		Port:        mp.SMTPPort,
		Encryption:  smtpprov.EncryptionNone,
		AuthMethod:  smtpprov.AuthMethodPlain,
		Username:    "test",
		Password:    "test",
		FromAddress: "noreply@tryanchor.dev",
		FromName:    "Anchor",
	})
	require.NoError(t, err)
	inst.ConfigJSON = goodCfg
	inst.ConfigVersion = 2
	_, err = IntegrationRepo.Update(context.Background(), tc.tenantID, inst)
	require.NoError(t, err)

	// Same dedupe key: must RE-DISPATCH the previously failed send, not return the
	// stale FAILED record.
	second, err := client.SendEmailWithResponse(context.Background(), tc.product.ProductID, body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, second.StatusCode())
	require.NotNil(t, second.JSON201)
	assert.Equal(t, "SENT", string(second.JSON201.Status), "re-dispatch flips the record to SENT")
	assert.NotNil(t, second.JSON201.SentAt)

	got := mp.WaitForMessage(t, 10*time.Second)
	assert.Equal(t, "bob@example.com", got.To[0].Address)
	assert.Len(t, mp.Messages(t), 1, "exactly one delivery — the re-dispatch")

	// The row was reused (dedupe unique index forbids a second): exactly one send
	// record for the key, now SENT.
	list, err := client.ListEmailSendsWithResponse(
		context.Background(), tc.product.ProductID, &ct.ListEmailSendsParams{Limit: ptr.Ptr(int64(50)), Offset: ptr.Ptr(int64(0))},
	)
	require.NoError(t, err)
	require.NotNil(t, list.JSON200)
	matching := 0
	for _, item := range list.JSON200.Items {
		if item.DedupeKey != nil && *item.DedupeKey == dedupeKey {
			matching++
			assert.Equal(t, "SENT", string(item.Status))
		}
	}
	assert.Equal(t, 1, matching, "the FAILED row was reused, not duplicated")
}

// createPublishedTemplate creates and publishes a minimal template, returning its id.
func createPublishedTemplate(t *testing.T, client *ct.ClientWithResponses, tc testCtx) string {
	t.Helper()
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
	require.NotNil(t, created.JSON201)
	tplID := created.JSON201.Id
	_, err = client.PublishEmailTemplateWithResponse(context.Background(), tc.product.ProductID, tplID)
	require.NoError(t, err)
	return tplID
}

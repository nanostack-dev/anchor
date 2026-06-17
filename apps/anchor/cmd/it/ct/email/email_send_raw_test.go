package email_ct_test

import (
	"context"
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
)

// TestEmailSendRaw exercises the templateless send path: callers that render
// their own subject/body (e.g. echopoint's scheduled-run failure alerts) send
// raw content without authoring a template first. dedupe_key remains the
// idempotency key, so a repeated raw send delivers at most once.
func TestEmailSendRaw(t *testing.T) {
	mp := mailpit.Start(t)
	defer mp.Stop(t)

	tc := newTestCtx(t)
	seedSMTPInstance(t, tc, mp)
	client := tc.product.OwnerAuthenticatedClient()

	t.Run("delivers raw content and returns SENT record with no template", func(t *testing.T) {
		mp.Reset(t)

		resp, sendErr := client.SendEmailWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.SendEmailJSONRequestBody{
				ToAddress: openapi_types.Email("raw@example.com"),
				ToName:    ptr.Ptr("Raw Recipient"),
				Subject:   ptr.Ptr("3 of 5 flows failed"),
				BodyHtml:  ptr.Ptr("<p>Monitor <b>Nightly</b> reported failures.</p>"),
				BodyText:  ptr.Ptr("Monitor Nightly reported failures."),
			},
		)
		require.NoError(t, sendErr)
		require.Equal(t, http.StatusCreated, resp.StatusCode())
		require.NotNil(t, resp.JSON201)
		assert.Equal(t, "SENT", string(resp.JSON201.Status))
		assert.NotNil(t, resp.JSON201.SentAt)
		assert.Equal(t, "raw@example.com", resp.JSON201.ToAddress)
		assert.Equal(t, "3 of 5 flows failed", resp.JSON201.Subject)
		assert.Nil(t, resp.JSON201.TemplateId, "raw send must not record a template")
		assert.Nil(t, resp.JSON201.TemplateVersionId)

		got := mp.WaitForMessage(t, 10*time.Second)
		body := mp.MessageByID(t, got.ID)
		assert.Equal(t, "raw@example.com", got.To[0].Address)
		assert.Contains(t, got.Subject, "3 of 5 flows failed")
		assert.Contains(t, body.HTML, "Nightly")
	})

	t.Run("same dedupe key returns identical record and single SMTP message", func(t *testing.T) {
		mp.Reset(t)

		dedupeKey := ids.MustNew("dkey")
		body := ct.SendEmailJSONRequestBody{
			ToAddress: openapi_types.Email("raw-dedupe@example.com"),
			Subject:   ptr.Ptr("schedule_run:abc:failure"),
			BodyText:  ptr.Ptr("1 of 1 flows failed"),
			DedupeKey: &dedupeKey,
		}

		first, firstErr := client.SendEmailWithResponse(context.Background(), tc.product.ProductID, body)
		require.NoError(t, firstErr)
		require.Equal(t, http.StatusCreated, first.StatusCode())

		mp.WaitForMessage(t, 10*time.Second)

		second, secondErr := client.SendEmailWithResponse(context.Background(), tc.product.ProductID, body)
		require.NoError(t, secondErr)
		require.Equal(t, http.StatusCreated, second.StatusCode())

		assert.Equal(t, first.JSON201.Id, second.JSON201.Id, "dedupe key must return same record")

		require.Eventually(t, func() bool {
			return len(mp.Messages(t)) == 1
		}, 2*time.Second, 100*time.Millisecond)
	})

	t.Run("rejects raw send missing subject or body", func(t *testing.T) {
		resp, sendErr := client.SendEmailWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.SendEmailJSONRequestBody{
				ToAddress: openapi_types.Email("incomplete@example.com"),
				Subject:   ptr.Ptr("Subject only, no body"),
			},
		)
		require.NoError(t, sendErr)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
		require.NotNil(t, resp.JSON400)
		assertAPIError(
			t,
			resp.JSON400.Errors,
			"EMAIL_CONTENT_MISSING",
			"A raw send requires subject and at least one of body_html or body_text",
		)
	})
}

package email_ct_test

import (
	"context"
	"net/http"
	"sync"
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

func TestEmailSendCreate(t *testing.T) {
	mp := mailpit.Start(t)
	defer mp.Stop(t)

	tc := newTestCtx(t)
	seedSMTPInstance(t, tc, mp)
	client := tc.product.OwnerAuthenticatedClient()

	// Seed a published template reused across sub-tests. Mailpit is the delivery
	// oracle for these checks: we assert that SMTP submission reaches it.
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

	t.Run("delivers email and returns SENT record", func(t *testing.T) {
		vars := map[string]interface{}{"name": "Bob"}
		resp, sendErr := client.SendEmailWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.SendEmailJSONRequestBody{
				TemplateId: &tplID,
				ToAddress:  openapi_types.Email("bob@example.com"),
				ToName:     ptr.Ptr("Bob"),
				Variables:  &vars,
			},
		)
		require.NoError(t, sendErr)
		assert.Equal(t, http.StatusCreated, resp.StatusCode())
		require.NotNil(t, resp.JSON201)
		assert.Equal(t, "SENT", string(resp.JSON201.Status))
		assert.NotNil(t, resp.JSON201.SentAt)
		assert.Equal(t, "bob@example.com", resp.JSON201.ToAddress)

		got := mp.WaitForMessage(t, 10*time.Second)
		body := mp.MessageByID(t, got.ID)
		assert.Equal(t, "bob@example.com", got.To[0].Address)
		assert.Contains(t, got.Subject, "Bob")
		assert.Contains(t, body.HTML, "Bob")
	})

	t.Run("same dedupe key returns identical record and single SMTP message", func(t *testing.T) {
		dedupeKey := ids.MustNew("dkey")
		vars := map[string]interface{}{}
		body := ct.SendEmailJSONRequestBody{
			TemplateId: &tplID,
			ToAddress:  openapi_types.Email("dedupe@example.com"),
			DedupeKey:  &dedupeKey,
			Variables:  &vars,
		}

		countBefore := len(mp.Messages(t))

		first, firstErr := client.SendEmailWithResponse(context.Background(), tc.product.ProductID, body)
		require.NoError(t, firstErr)
		require.Equal(t, http.StatusCreated, first.StatusCode())

		mp.WaitForMessage(t, 10*time.Second)

		second, secondErr := client.SendEmailWithResponse(context.Background(), tc.product.ProductID, body)
		require.NoError(t, secondErr)
		require.Equal(t, http.StatusCreated, second.StatusCode())

		assert.Equal(t, first.JSON201.Id, second.JSON201.Id, "dedupe key must return same record")

		time.Sleep(200 * time.Millisecond)
		assert.Len(t, mp.Messages(t), countBefore+1, "only one SMTP message despite two sends")
	})

	t.Run("concurrent same dedupe key returns identical record and single SMTP message", func(t *testing.T) {
		mp.Reset(t)

		dedupeKey := ids.MustNew("dkey")
		vars := map[string]interface{}{"name": "Concurrent"}
		body := ct.SendEmailJSONRequestBody{
			TemplateId: &tplID,
			ToAddress:  openapi_types.Email("dedupe-concurrent@example.com"),
			DedupeKey:  &dedupeKey,
			Variables:  &vars,
		}

		const parallelRequests = 6
		start := make(chan struct{})
		responses := make(chan *ct.SendEmailResponse, parallelRequests)
		errorsCh := make(chan error, parallelRequests)

		var wg sync.WaitGroup
		for range parallelRequests {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				resp, sendErr := client.SendEmailWithResponse(context.Background(), tc.product.ProductID, body)
				if sendErr != nil {
					errorsCh <- sendErr
					return
				}
				responses <- resp
			}()
		}

		close(start)
		wg.Wait()
		close(errorsCh)
		close(responses)

		for sendErr := range errorsCh {
			require.NoError(t, sendErr)
		}

		responseCount := 0
		firstID := ""
		for resp := range responses {
			responseCount++
			require.Equal(t, http.StatusCreated, resp.StatusCode())
			require.NotNil(t, resp.JSON201)
			if firstID == "" {
				firstID = resp.JSON201.Id
				continue
			}
			assert.Equal(t, firstID, resp.JSON201.Id)
		}
		require.Equal(t, parallelRequests, responseCount)

		got := mp.WaitForMessage(t, 10*time.Second)
		assert.Equal(t, "dedupe-concurrent@example.com", got.To[0].Address)
		require.Eventually(t, func() bool {
			return len(mp.Messages(t)) == 1
		}, 2*time.Second, 100*time.Millisecond)
	})

	t.Run("rejects send to unpublished template", func(t *testing.T) {
		draft, createErr := client.CreateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:     uniqueSlug(),
				Name:     "Unpublished",
				Subject:  "Subj",
				BodyHtml: "<p>Body</p>",
			},
		)
		require.NoError(t, createErr)
		draftID := draft.JSON201.Id

		vars := map[string]interface{}{}
		resp, sendErr := client.SendEmailWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.SendEmailJSONRequestBody{
				TemplateId: &draftID,
				ToAddress:  openapi_types.Email("user@example.com"),
				Variables:  &vars,
			},
		)
		require.NoError(t, sendErr)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
		require.NotNil(t, resp.JSON400)
		assertAPIError(
			t,
			resp.JSON400.Errors,
			"EMAIL_TEMPLATE_NOT_PUBLISHED",
			"This template has no published version; publish it before sending",
		)
	})
}

package email_ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/cmd/it/shared/mailpit"
)

func TestEmailSendList(t *testing.T) {
	t.Run("returns empty list for new product", func(t *testing.T) {
		tc := newTestCtx(t)
		resp, err := tc.product.OwnerAuthenticatedClient().ListEmailSendsWithResponse(
			context.Background(),
			tc.product.ProductID,
			&ct.ListEmailSendsParams{},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		assert.Equal(t, 0, resp.JSON200.Count)
	})

	t.Run("returns send records after successful send", func(t *testing.T) {
		mp := mailpit.Start(t)
		defer mp.Stop(t)

		tc := newTestCtx(t)
		seedSMTPInstance(t, tc, mp)
		client := tc.product.OwnerAuthenticatedClient()

		created, err := client.CreateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:     uniqueSlug(),
				Name:     "List Test",
				Subject:  "Hi",
				BodyHtml: "<p>Hi</p>",
			},
		)
		require.NoError(t, err)
		tplID := created.JSON201.Id

		_, err = client.PublishEmailTemplateWithResponse(context.Background(), tc.product.ProductID, tplID)
		require.NoError(t, err)

		vars := map[string]any{}
		_, err = client.SendEmailWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.SendEmailJSONRequestBody{
				TemplateId: &tplID,
				ToAddress:  openapi_types.Email("user@example.com"),
				Variables:  &vars,
			},
		)
		require.NoError(t, err)
		mp.WaitForMessage(t, 10*time.Second)

		list, err := client.ListEmailSendsWithResponse(
			context.Background(),
			tc.product.ProductID,
			&ct.ListEmailSendsParams{},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, list.StatusCode())
		assert.Equal(t, 1, list.JSON200.Count)
		assert.Equal(t, "SENT", string(list.JSON200.Items[0].Status))
	})
}

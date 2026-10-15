package ct_test

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

// Not parallel: it resets the shared mailpit inbox.
func TestEmailSendList(t *testing.T) {
	t.Run("returns empty list for new product", func(t *testing.T) {
		tc := newEmailTestCtx(t)
		resp, err := tc.product.OwnerAuthenticatedClient().SearchEmailSendsWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.SearchEmailSendsJSONRequestBody{},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		assert.Equal(t, 0, resp.JSON200.Count)
		assert.Empty(t, resp.JSON200.NextCursor)
	})

	t.Run("returns send records after successful send", func(t *testing.T) {
		mp := mailpit.Shared(t)

		tc := newEmailTestCtx(t)
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

		list, err := client.SearchEmailSendsWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.SearchEmailSendsJSONRequestBody{},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, list.StatusCode())
		assert.Equal(t, 1, list.JSON200.Count)
		assert.EqualValues(t, 1, list.JSON200.Total)
		assert.Equal(t, "SENT", string(list.JSON200.Items[0].Status))
	})
}

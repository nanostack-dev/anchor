package email_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailTemplateDraftUpdate(t *testing.T) {
	tc := newTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()

	created, err := client.CreateEmailTemplateWithResponse(
		context.Background(),
		tc.product.ProductID,
		ct.CreateEmailTemplateJSONRequestBody{
			Slug:     uniqueSlug(),
			Name:     "Draft Template",
			Subject:  "Original {{ .name }}",
			BodyHtml: "<p>Original {{ .name }}</p>",
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, created.StatusCode())
	tplID := created.JSON201.Id

	t.Run("updates subject and body", func(t *testing.T) {
		resp, updateErr := client.UpdateEmailTemplateDraftWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
			ct.UpdateEmailTemplateDraftJSONRequestBody{
				Subject:  new("Updated {{ .name }}"),
				BodyHtml: new("<p>Updated {{ .name }}</p>"),
			},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, "DRAFT", string(resp.JSON200.Status))
		assert.Equal(t, "Updated {{ .name }}", resp.JSON200.Subject)
	})

	t.Run("returns 404 for unknown template", func(t *testing.T) {
		resp, updateErr := client.UpdateEmailTemplateDraftWithResponse(
			context.Background(),
			tc.product.ProductID,
			ids.MustNew("etpl"),
			ct.UpdateEmailTemplateDraftJSONRequestBody{Subject: new("x")},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

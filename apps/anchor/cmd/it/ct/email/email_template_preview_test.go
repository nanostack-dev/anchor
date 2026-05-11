package email_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/shared/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailTemplatePreview(t *testing.T) {
	tc := newTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()

	created, err := client.CreateEmailTemplateWithResponse(
		context.Background(),
		tc.product.ProductID,
		ct.CreateEmailTemplateJSONRequestBody{
			Slug:     uniqueSlug(),
			Name:     "Preview Template",
			Subject:  "Hello {{ .name }}",
			BodyHtml: "<p>Hi {{ .name }}</p>",
			BodyText: toolkit.Ptr("Hi {{ .name }}"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, created.StatusCode())
	tplID := created.JSON201.Id

	t.Run("renders draft with variables", func(t *testing.T) {
		vars := map[string]interface{}{"name": "Alice"}
		resp, previewErr := client.PreviewEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
			ct.PreviewEmailTemplateJSONRequestBody{Variables: &vars},
		)
		require.NoError(t, previewErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, "Hello Alice", resp.JSON200.Subject)
		assert.Contains(t, resp.JSON200.BodyHtml, "Alice")
		assert.Contains(t, resp.JSON200.BodyText, "Alice")
	})

	t.Run("renders without variables (empty substitution)", func(t *testing.T) {
		resp, previewErr := client.PreviewEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
			ct.PreviewEmailTemplateJSONRequestBody{},
		)
		require.NoError(t, previewErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("returns 404 for unknown template", func(t *testing.T) {
		vars := map[string]interface{}{}
		resp, previewErr := client.PreviewEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			toolkit.NewID("etpl"),
			ct.PreviewEmailTemplateJSONRequestBody{Variables: &vars},
		)
		require.NoError(t, previewErr)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

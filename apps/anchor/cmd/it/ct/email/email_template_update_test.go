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

func TestEmailTemplateUpdate(t *testing.T) {
	tc := newTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()

	created, err := client.CreateEmailTemplateWithResponse(
		context.Background(),
		tc.product.ProductID,
		ct.CreateEmailTemplateJSONRequestBody{
			Slug:     uniqueSlug(),
			Name:     "Original",
			Subject:  "Subj",
			BodyHtml: "<p>Body</p>",
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, created.StatusCode())
	tplID := created.JSON201.Id

	t.Run("updates name", func(t *testing.T) {
		resp, updateErr := client.UpdateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
			ct.UpdateEmailTemplateJSONRequestBody{Name: new("Renamed")},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, "Renamed", resp.JSON200.Name)
	})

	t.Run("updates description", func(t *testing.T) {
		resp, updateErr := client.UpdateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
			ct.UpdateEmailTemplateJSONRequestBody{Description: new("new desc")},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
	})

	t.Run("deactivates template", func(t *testing.T) {
		resp, updateErr := client.UpdateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
			ct.UpdateEmailTemplateJSONRequestBody{IsActive: new(false)},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.False(t, resp.JSON200.IsActive)
	})

	t.Run("returns 404 for unknown ID", func(t *testing.T) {
		resp, updateErr := client.UpdateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ids.MustNew("etpl"),
			ct.UpdateEmailTemplateJSONRequestBody{Name: new("x")},
		)
		require.NoError(t, updateErr)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

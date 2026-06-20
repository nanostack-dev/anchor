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

func TestEmailTemplateGet(t *testing.T) {
	tc := newTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()

	created, err := client.CreateEmailTemplateWithResponse(
		context.Background(),
		tc.product.ProductID,
		ct.CreateEmailTemplateJSONRequestBody{
			Slug:        uniqueSlug(),
			Name:        "Get Template",
			Subject:     "Subj",
			BodyHtml:    "<p>Body</p>",
			Description: new("desc"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, created.StatusCode())
	tplID := created.JSON201.Id

	t.Run("returns existing template", func(t *testing.T) {
		resp, getErr := client.GetEmailTemplateWithResponse(context.Background(), tc.product.ProductID, tplID)
		require.NoError(t, getErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, tplID, resp.JSON200.Id)
		assert.NotNil(t, resp.JSON200.DraftVersionId)
		assert.Nil(t, resp.JSON200.PublishedVersionId)
	})

	t.Run("returns 404 for unknown ID", func(t *testing.T) {
		resp, getErr := client.GetEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ids.MustNew("etpl"),
		)
		require.NoError(t, getErr)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})

	t.Run("returns 404 when accessing template from another product", func(t *testing.T) {
		other := newTestCtx(t)
		resp, getErr := other.product.OwnerAuthenticatedClient().GetEmailTemplateWithResponse(
			context.Background(),
			other.product.ProductID,
			tplID,
		)
		require.NoError(t, getErr)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

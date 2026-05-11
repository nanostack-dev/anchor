package email_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailTemplateList(t *testing.T) {
	t.Run("returns empty list for new product", func(t *testing.T) {
		tc := newTestCtx(t)
		resp, err := tc.product.OwnerAuthenticatedClient().ListEmailTemplatesWithResponse(
			context.Background(),
			tc.product.ProductID,
			&ct.ListEmailTemplatesParams{},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, 0, resp.JSON200.Count)
		assert.Empty(t, resp.JSON200.Items)
	})

	t.Run("returns all created templates", func(t *testing.T) {
		tc := newTestCtx(t)
		client := tc.product.OwnerAuthenticatedClient()

		for range 3 {
			_, err := client.CreateEmailTemplateWithResponse(
				context.Background(),
				tc.product.ProductID,
				ct.CreateEmailTemplateJSONRequestBody{
					Slug:     uniqueSlug(),
					Name:     "Template",
					Subject:  "Subj",
					BodyHtml: "<p>Body</p>",
				},
			)
			require.NoError(t, err)
		}

		resp, err := client.ListEmailTemplatesWithResponse(
			context.Background(),
			tc.product.ProductID,
			&ct.ListEmailTemplatesParams{},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		assert.Equal(t, 3, resp.JSON200.Count)
		assert.Len(t, resp.JSON200.Items, 3)
	})

	t.Run("templates are isolated per product", func(t *testing.T) {
		tc1 := newTestCtx(t)
		tc2 := newTestCtx(t)

		_, err := tc1.product.OwnerAuthenticatedClient().CreateEmailTemplateWithResponse(
			context.Background(),
			tc1.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:     uniqueSlug(),
				Name:     "Product 1 Template",
				Subject:  "Subj",
				BodyHtml: "<p>Body</p>",
			},
		)
		require.NoError(t, err)

		resp, err := tc2.product.OwnerAuthenticatedClient().ListEmailTemplatesWithResponse(
			context.Background(),
			tc2.product.ProductID,
			&ct.ListEmailTemplatesParams{},
		)
		require.NoError(t, err)
		assert.Equal(t, 0, resp.JSON200.Count)
	})
}

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

func TestEmailTemplateDelete(t *testing.T) {
	tc := newTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()

	t.Run("deletes existing template", func(t *testing.T) {
		created, err := client.CreateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:     uniqueSlug(),
				Name:     "Delete Me",
				Subject:  "Subj",
				BodyHtml: "<p>Body</p>",
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode())
		tplID := created.JSON201.Id

		delResp, err := client.DeleteEmailTemplateWithResponse(context.Background(), tc.product.ProductID, tplID)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, delResp.StatusCode())

		getResp, err := client.GetEmailTemplateWithResponse(context.Background(), tc.product.ProductID, tplID)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode())
	})

	t.Run("returns 404 for unknown ID", func(t *testing.T) {
		resp, err := client.DeleteEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			toolkit.NewID("etpl"),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})

	t.Run("deleting one template does not affect others", func(t *testing.T) {
		a, err := client.CreateEmailTemplateWithResponse(
			context.Background(), tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{Slug: uniqueSlug(), Name: "A", Subject: "S", BodyHtml: "<p>B</p>"},
		)
		require.NoError(t, err)
		b, err := client.CreateEmailTemplateWithResponse(
			context.Background(), tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{Slug: uniqueSlug(), Name: "B", Subject: "S", BodyHtml: "<p>B</p>"},
		)
		require.NoError(t, err)

		_, err = client.DeleteEmailTemplateWithResponse(context.Background(), tc.product.ProductID, a.JSON201.Id)
		require.NoError(t, err)

		getB, err := client.GetEmailTemplateWithResponse(context.Background(), tc.product.ProductID, b.JSON201.Id)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, getB.StatusCode())
	})
}

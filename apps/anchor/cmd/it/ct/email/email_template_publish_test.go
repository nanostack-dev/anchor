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

func TestEmailTemplatePublish(t *testing.T) {
	tc := newTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()

	t.Run("publishes draft and updates template pointer", func(t *testing.T) {
		created, err := client.CreateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:     uniqueSlug(),
				Name:     "Publish Me",
				Subject:  "Hello",
				BodyHtml: "<p>Hello</p>",
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode())
		tplID := created.JSON201.Id
		require.Nil(t, created.JSON201.PublishedVersionId)

		pub, publishErr := client.PublishEmailTemplateWithResponse(context.Background(), tc.product.ProductID, tplID)
		require.NoError(t, publishErr)
		assert.Equal(t, http.StatusOK, pub.StatusCode())
		require.NotNil(t, pub.JSON200)
		assert.Equal(t, ct.EmailTemplateVersionStatusPUBLISHED, pub.JSON200.Status)
		assert.NotNil(t, pub.JSON200.PublishedAt)

		get, getErr := client.GetEmailTemplateWithResponse(context.Background(), tc.product.ProductID, tplID)
		require.NoError(t, getErr)
		assert.NotNil(t, get.JSON200.PublishedVersionId)
		assert.Equal(t, pub.JSON200.Id, *get.JSON200.PublishedVersionId)
	})

	t.Run("second publish creates new published version and opens fresh draft", func(t *testing.T) {
		created, err := client.CreateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:     uniqueSlug(),
				Name:     "Republish Me",
				Subject:  "v1",
				BodyHtml: "<p>v1</p>",
			},
		)
		require.NoError(t, err)
		tplID := created.JSON201.Id

		first, firstPublishErr := client.PublishEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
		)
		require.NoError(t, firstPublishErr)
		require.Equal(t, http.StatusOK, first.StatusCode())

		_, err = client.UpdateEmailTemplateDraftWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
			ct.UpdateEmailTemplateDraftJSONRequestBody{Subject: new("v2")},
		)
		require.NoError(t, err)

		second, secondPublishErr := client.PublishEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			tplID,
		)
		require.NoError(t, secondPublishErr)
		assert.Equal(t, http.StatusOK, second.StatusCode())
		assert.NotEqual(t, first.JSON200.Id, second.JSON200.Id)
		assert.Equal(t, "v2", second.JSON200.Subject)
	})

	t.Run("returns 404 for unknown template", func(t *testing.T) {
		resp, publishErr := client.PublishEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ids.MustNew("etpl"),
		)
		require.NoError(t, publishErr)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

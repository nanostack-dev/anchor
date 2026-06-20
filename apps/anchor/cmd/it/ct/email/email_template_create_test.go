package email_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailTemplateCreate(t *testing.T) {
	tc := newTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()

	t.Run("creates template with draft version", func(t *testing.T) {
		resp, err := client.CreateEmailTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug:        uniqueSlug(),
				Name:        "Welcome Email",
				Subject:     "Welcome {{ .name }}",
				BodyHtml:    "<p>Hi {{ .name }}</p>",
				Description: new("onboarding template"),
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode())
		require.NotNil(t, resp.JSON201)
		tpl := resp.JSON201
		assert.NotEmpty(t, tpl.Id)
		assert.NotEmpty(t, tpl.Slug)
		assert.Equal(t, "Welcome Email", tpl.Name)
		assert.NotNil(t, tpl.DraftVersionId)
		assert.Nil(t, tpl.PublishedVersionId)
		assert.True(t, tpl.IsActive)
	})

	t.Run("rejects duplicate slug", func(t *testing.T) {
		slug := uniqueSlug()
		body := ct.CreateEmailTemplateJSONRequestBody{
			Slug:     slug,
			Name:     "First",
			Subject:  "Subj",
			BodyHtml: "<p>Body</p>",
		}
		first, err := client.CreateEmailTemplateWithResponse(context.Background(), tc.product.ProductID, body)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, first.StatusCode())

		body.Name = "Second"
		second, err := client.CreateEmailTemplateWithResponse(context.Background(), tc.product.ProductID, body)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, second.StatusCode())
		require.NotNil(t, second.JSON400)
		assertAPIError(
			t,
			second.JSON400.Errors,
			"EMAIL_TEMPLATE_SLUG_TAKEN",
			"An email template with this slug already exists for the product",
		)
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		cases := []struct {
			name string
			body ct.CreateEmailTemplateJSONRequestBody
		}{
			{
				name: "missing slug",
				body: ct.CreateEmailTemplateJSONRequestBody{Name: "N", Subject: "S", BodyHtml: "<p>B</p>"},
			},
			{
				name: "missing name",
				body: ct.CreateEmailTemplateJSONRequestBody{Slug: uniqueSlug(), Subject: "S", BodyHtml: "<p>B</p>"},
			},
			{
				name: "missing subject",
				body: ct.CreateEmailTemplateJSONRequestBody{Slug: uniqueSlug(), Name: "N", BodyHtml: "<p>B</p>"},
			},
			{
				name: "missing body_html",
				body: ct.CreateEmailTemplateJSONRequestBody{Slug: uniqueSlug(), Name: "N", Subject: "S"},
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				resp, err := client.CreateEmailTemplateWithResponse(context.Background(), tc.product.ProductID, c.body)
				require.NoError(t, err)
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
			})
		}
	})
}

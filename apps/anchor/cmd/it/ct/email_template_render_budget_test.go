package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailTemplatePreviewEnforcesRuntimeStepBudget(t *testing.T) {
	t.Parallel()
	tc := newEmailTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()

	cases := []struct {
		name     string
		bodyHTML string
		vars     map[string]any
	}{
		{
			name:     "variable-indirected integer range is refused quickly",
			bodyHTML: "{{$n := 100000000}}{{range $n}}{{end}}",
			vars:     map[string]any{},
		},
		{
			name:     "nested quadratic data range is refused quickly",
			bodyHTML: "{{range .items}}{{range $.items}}{{end}}{{end}}",
			vars:     map[string]any{"items": make([]int, 200)},
		},
		{
			name: "silent exponential define fan-out is refused quickly",
			bodyHTML: `{{define "a0"}}{{end}}` +
				`{{define "a1"}}{{template "a0"}}{{template "a0"}}{{end}}` +
				`{{define "a2"}}{{template "a1"}}{{template "a1"}}{{end}}` +
				`{{define "a3"}}{{template "a2"}}{{template "a2"}}{{end}}` +
				`{{define "a4"}}{{template "a3"}}{{template "a3"}}{{end}}` +
				`{{define "a5"}}{{template "a4"}}{{template "a4"}}{{end}}` +
				`{{define "a6"}}{{template "a5"}}{{template "a5"}}{{end}}` +
				`{{define "a7"}}{{template "a6"}}{{template "a6"}}{{end}}` +
				`{{define "a8"}}{{template "a7"}}{{template "a7"}}{{end}}` +
				`{{define "a9"}}{{template "a8"}}{{template "a8"}}{{end}}` +
				`{{define "a10"}}{{template "a9"}}{{template "a9"}}{{end}}` +
				`{{define "a11"}}{{template "a10"}}{{template "a10"}}{{end}}` +
				`{{define "a12"}}{{template "a11"}}{{template "a11"}}{{end}}` +
				`{{define "a13"}}{{template "a12"}}{{template "a12"}}{{end}}` +
				`{{define "a14"}}{{template "a13"}}{{template "a13"}}{{end}}` +
				`{{define "a15"}}{{template "a14"}}{{template "a14"}}{{end}}` +
				`{{template "a15"}}`,
			vars: map[string]any{},
		},
	}

	for _, sc := range cases {
		t.Run(sc.name, func(t *testing.T) {
			created, err := client.CreateEmailTemplateWithResponse(
				context.Background(),
				tc.product.ProductID,
				ct.CreateEmailTemplateJSONRequestBody{
					Slug:     uniqueSlug(),
					Name:     "Budget " + sc.name,
					Subject:  "Budget Test",
					BodyHtml: sc.bodyHTML,
					BodyText: new("plain"),
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))

			start := time.Now()
			resp, previewErr := client.PreviewEmailTemplateWithResponse(
				context.Background(),
				tc.product.ProductID,
				created.JSON201.Id,
				ct.PreviewEmailTemplateJSONRequestBody{Variables: &sc.vars},
			)
			require.NoError(t, previewErr)
			assert.Less(t, time.Since(start), 5*time.Second)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
			require.NotNil(t, resp.JSON400)
			require.NotEmpty(t, resp.JSON400.Errors)
			assert.Equal(t, "EMAIL_TEMPLATE_RENDER_ERROR", resp.JSON400.Errors[0].Code)
		})
	}
}

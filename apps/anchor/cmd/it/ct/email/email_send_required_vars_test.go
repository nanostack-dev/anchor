package email_ct_test

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

// TestEmailSendRequiredVariables verifies that a send referencing a template
// whose schema marks a variable required is rejected when that variable is
// absent, and succeeds once it is supplied.
func TestEmailSendRequiredVariables(t *testing.T) {
	mp := mailpit.Start(t)
	defer mp.Stop(t)

	tc := newTestCtx(t)
	seedSMTPInstance(t, tc, mp)
	client := tc.product.OwnerAuthenticatedClient()

	created, err := client.CreateEmailTemplateWithResponse(
		context.Background(),
		tc.product.ProductID,
		ct.CreateEmailTemplateJSONRequestBody{
			Slug:     uniqueSlug(),
			Name:     "Welcome",
			Subject:  "Hello {{ .name }}",
			BodyHtml: "<p>Hi {{ .name }}</p>",
			Variables: &[]ct.EmailVariableSchema{
				{Name: "name", Type: ct.EmailVariableTypeSTRING, Required: new(true)},
			},
		},
	)
	require.NoError(t, err)
	tplID := created.JSON201.Id

	_, err = client.PublishEmailTemplateWithResponse(context.Background(), tc.product.ProductID, tplID)
	require.NoError(t, err)

	t.Run("rejects send missing a required variable", func(t *testing.T) {
		countBefore := len(mp.Messages(t))

		resp, sendErr := client.SendEmailWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.SendEmailJSONRequestBody{
				TemplateId: &tplID,
				ToAddress:  openapi_types.Email("nobody@example.com"),
			},
		)
		require.NoError(t, sendErr)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
		require.NotNil(t, resp.JSON400)
		require.Len(t, resp.JSON400.Errors, 1)
		assert.Equal(t, "EMAIL_REQUIRED_VARIABLES_MISSING", resp.JSON400.Errors[0].Code)
		assert.Contains(t, resp.JSON400.Errors[0].Message, "name")

		time.Sleep(200 * time.Millisecond)
		assert.Len(t, mp.Messages(t), countBefore, "rejected send must not deliver")
	})

	t.Run("delivers once the required variable is supplied", func(t *testing.T) {
		vars := map[string]any{"name": "Ada"}
		resp, sendErr := client.SendEmailWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.SendEmailJSONRequestBody{
				TemplateId: &tplID,
				ToAddress:  openapi_types.Email("ada@example.com"),
				Variables:  &vars,
			},
		)
		require.NoError(t, sendErr)
		require.Equal(t, http.StatusCreated, resp.StatusCode())
		require.NotNil(t, resp.JSON201)
		assert.Equal(t, "SENT", string(resp.JSON201.Status))

		got := mp.WaitForMessage(t, 10*time.Second)
		assert.Contains(t, got.Subject, "Ada")
	})
}

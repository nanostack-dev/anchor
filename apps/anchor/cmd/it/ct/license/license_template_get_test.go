package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseTemplateGet(t *testing.T) {
	t.Run("reads a template back", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		resp, err := tc.product.OwnerAuthenticatedClient().GetLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, created.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		assert.Equal(t, created.Id, resp.JSON200.Id)
		assert.Equal(t, created.Name, resp.JSON200.Name)
		assert.InDelta(t, 500.0, resp.JSON200.Values["flows"], 0)
	})

	t.Run("404 when the product has no template with that identifier", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := tc.product.OwnerAuthenticatedClient().GetLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, missingTemplateID(),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

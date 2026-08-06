package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseTemplateDelete(t *testing.T) {
	t.Run("removes the template", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		del, err := tc.product.OwnerAuthenticatedClient().DeleteLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, created.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, del.StatusCode(), string(del.Body))

		read, err := tc.product.OwnerAuthenticatedClient().GetLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, created.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, read.StatusCode(), string(read.Body))
	})

	t.Run("frees the name for reuse", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, "Pro", validTemplateValues())

		del, err := tc.product.OwnerAuthenticatedClient().DeleteLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, created.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, del.StatusCode(), string(del.Body))

		reused := createTemplate(t, tc, "Pro", validTemplateValues())
		assert.NotEqual(t, created.Id, reused.Id)
	})

	t.Run("404 when the product has no template with that identifier", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := tc.product.OwnerAuthenticatedClient().DeleteLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, missingTemplateID(),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseTemplateList(t *testing.T) {
	t.Run("lists the product's templates by name", func(t *testing.T) {
		tc := newTemplateCtx(t)
		createTemplate(t, tc, "Pro", validTemplateValues())
		createTemplate(t, tc, "Free", ct.LicenseTemplateValues{"flows": 10, "sso": false})

		resp, err := tc.product.OwnerAuthenticatedClient().ListLicenseTemplatesWithResponse(
			context.Background(), tc.product.ProductID,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		require.Len(t, resp.JSON200.Items, 2)
		assert.Equal(t, 2, resp.JSON200.Count)
		assert.Equal(t, []string{"Free", "Pro"}, templateNames(resp.JSON200.Items))
	})

	t.Run("lists nothing for a product with no templates", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := tc.product.OwnerAuthenticatedClient().ListLicenseTemplatesWithResponse(
			context.Background(), tc.product.ProductID,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)
		assert.Empty(t, resp.JSON200.Items)
		assert.Equal(t, 0, resp.JSON200.Count)
	})

	t.Run("does not list another product's templates", func(t *testing.T) {
		first := newTemplateCtx(t)
		second := newTemplateCtx(t)
		createTemplate(t, first, "Pro", validTemplateValues())

		resp, err := second.product.OwnerAuthenticatedClient().ListLicenseTemplatesWithResponse(
			context.Background(), second.product.ProductID,
		)
		require.NoError(t, err)
		require.NotNil(t, resp.JSON200)
		assert.Empty(t, resp.JSON200.Items)
	})
}

package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseTemplateList(t *testing.T) {
	// Checks license template listing across the scenarios below.
	t.Parallel()
	t.Run("lists the product's templates by name", func(t *testing.T) {
		// The API lists the product's templates by name.
		tc := newTemplateCtx(t)
		createTemplate(t, tc, "Pro", validTemplateValues())
		createTemplate(t, tc, "Free", templateValuesWith("flows", 10))

		resp, err := tc.product.OwnerAuthenticatedClient().SearchLicenseTemplatesWithResponse(
			context.Background(), tc.product.ProductID, ct.SearchLicenseTemplatesJSONRequestBody{},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		require.Len(t, resp.JSON200.Items, 2)
		assert.Equal(t, 2, resp.JSON200.Count)
		assert.Equal(t, []string{"Free", "Pro"}, templateNames(resp.JSON200.Items))
	})

	t.Run("lists nothing for a product with no templates", func(t *testing.T) {
		// The API lists nothing for a product with no templates.
		tc := newTemplateCtx(t)

		resp, err := tc.product.OwnerAuthenticatedClient().SearchLicenseTemplatesWithResponse(
			context.Background(), tc.product.ProductID, ct.SearchLicenseTemplatesJSONRequestBody{},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)
		assert.Empty(t, resp.JSON200.Items)
		assert.Equal(t, 0, resp.JSON200.Count)
	})

	t.Run("does not list another product's templates", func(t *testing.T) {
		// The API does not list another product's templates.
		first := newTemplateCtx(t)
		second := newTemplateCtx(t)
		createTemplate(t, first, "Pro", validTemplateValues())

		resp, err := second.product.OwnerAuthenticatedClient().SearchLicenseTemplatesWithResponse(
			context.Background(), second.product.ProductID, ct.SearchLicenseTemplatesJSONRequestBody{},
		)
		require.NoError(t, err)
		require.NotNil(t, resp.JSON200)
		assert.Empty(t, resp.JSON200.Items)
	})
}

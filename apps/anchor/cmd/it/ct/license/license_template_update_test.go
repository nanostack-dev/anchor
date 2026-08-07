package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseTemplateUpdate(t *testing.T) {
	t.Run("renames without touching the values", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			created.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Name: new("Enterprise")},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		assert.Equal(t, "Enterprise", resp.JSON200.Name)
		assert.InDelta(t, 500.0, resp.JSON200.Values["flows"], 0)
		assert.Equal(t, true, resp.JSON200.Values["sso"])
	})

	t.Run("replaces the values wholesale", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())

		// Every declared field is restated, because a request that dropped one
		// would be a removal and a removal is refused. What "wholesale" buys here
		// is that the stored set is the request's, not a merge with what was
		// there — so every value moves at once.
		replacement := ct.LicenseTemplateValues{
			"flows":        900,
			"sso":          false,
			"support_tier": "basic",
			"region":       "eu-west",
		}
		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			created.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Values: &replacement},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		assert.InDelta(t, 900.0, resp.JSON200.Values["flows"], 0)
		assert.Equal(t, false, resp.JSON200.Values["sso"])
		assert.Equal(t, "basic", resp.JSON200.Values["support_tier"])
		assert.Equal(t, "eu-west", resp.JSON200.Values["region"])
	})

	t.Run("404 when the product has no template with that identifier", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			missingTemplateID(),
			ct.UpdateLicenseTemplateJSONRequestBody{Name: new("Enterprise")},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

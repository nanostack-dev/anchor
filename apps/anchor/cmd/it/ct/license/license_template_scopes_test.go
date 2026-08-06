package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLicenseTemplateScopes checks each route against a Product API key holding
// every other license template scope. The route security matrix already proves
// the contract declares security; this proves the four scopes are genuinely
// distinct, so a read-only key cannot rewrite what a tier grants.
func TestLicenseTemplateScopes(t *testing.T) {
	t.Run("read scope cannot write", func(t *testing.T) {
		tc := newTemplateCtx(t)
		created := createTemplate(t, tc, uniqueTemplateName(), validTemplateValues())
		readOnly, _ := tc.product.CreateAPIKeyClientWithScopes([]string{"license_template:read"})

		create, err := readOnly.CreateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseTemplateJSONRequestBody{
				Name:   uniqueTemplateName(),
				Values: validTemplateValues(),
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, create.StatusCode())

		update, err := readOnly.UpdateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			created.Id,
			ct.UpdateLicenseTemplateJSONRequestBody{Name: new("Enterprise")},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, update.StatusCode())

		del, err := readOnly.DeleteLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, created.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, del.StatusCode())
	})

	t.Run("write scopes cannot read", func(t *testing.T) {
		tc := newTemplateCtx(t)
		writeOnly, _ := tc.product.CreateAPIKeyClientWithScopes(
			[]string{"license_template:create", "license_template:update", "license_template:delete"},
		)

		created, err := writeOnly.CreateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseTemplateJSONRequestBody{
				Name:   uniqueTemplateName(),
				Values: validTemplateValues(),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))
		require.NotNil(t, created.JSON201)

		read, err := writeOnly.GetLicenseTemplateWithResponse(
			context.Background(), tc.product.ProductID, created.JSON201.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, read.StatusCode())

		list, err := writeOnly.ListLicenseTemplatesWithResponse(
			context.Background(), tc.product.ProductID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, list.StatusCode())
	})

	t.Run("a license schema scope does not reach templates", func(t *testing.T) {
		tc := newTemplateCtx(t)
		// The two resources are separate entries in the permission catalog, so a
		// key trusted to declare what a license may contain is not thereby
		// trusted to decide what a tier grants.
		schemaOnly, _ := tc.product.CreateAPIKeyClientWithScopes(
			[]string{"license_schema:read", "license_schema:create", "license_schema:update"},
		)

		list, err := schemaOnly.ListLicenseTemplatesWithResponse(
			context.Background(), tc.product.ProductID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, list.StatusCode())

		create, err := schemaOnly.CreateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseTemplateJSONRequestBody{
				Name:   uniqueTemplateName(),
				Values: validTemplateValues(),
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, create.StatusCode())
	})
}

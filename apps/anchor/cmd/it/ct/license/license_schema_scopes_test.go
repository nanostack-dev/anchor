package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLicenseSchemaScopes checks each route against a Product API key that
// holds every other licensing scope. The route security matrix already proves
// the contract declares security; this proves the four scopes are genuinely
// distinct, so a read-only key cannot rewrite a declaration.
func TestLicenseSchemaScopes(t *testing.T) {
	t.Run("read scope cannot write", func(t *testing.T) {
		tc := newTestCtx(t)
		readOnly, _ := tc.product.CreateAPIKeyClientWithScopes([]string{"license_schema:read"})

		create, err := readOnly.CreateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseSchemaJSONRequestBody{Fields: []ct.LicenseFieldDeclaration{}},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, create.StatusCode())

		update, err := readOnly.UpdateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.UpdateLicenseSchemaJSONRequestBody{Description: new("nope")},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, update.StatusCode())

		del, err := readOnly.DeleteLicenseSchemaWithResponse(context.Background(), tc.product.ProductID)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, del.StatusCode())
	})

	t.Run("write scopes cannot read", func(t *testing.T) {
		tc := newTestCtx(t)
		writeOnly, _ := tc.product.CreateAPIKeyClientWithScopes(
			[]string{"license_schema:create", "license_schema:update", "license_schema:delete"},
		)

		created, err := writeOnly.CreateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseSchemaJSONRequestBody{
				Fields: []ct.LicenseFieldDeclaration{{Name: "flows", Type: ct.LicenseFieldTypeLimit}},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))

		read, err := writeOnly.GetLicenseSchemaWithResponse(context.Background(), tc.product.ProductID)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, read.StatusCode())
	})
}

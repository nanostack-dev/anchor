package license_ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

func TestLicenseSchemaDelete(t *testing.T) {
	t.Run("removes the schema and its fields", func(t *testing.T) {
		tc := newTestCtx(t)
		client := seedSchema(t, tc)

		resp, err := client.DeleteLicenseSchemaWithResponse(context.Background(), tc.product.ProductID)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

		read, err := client.GetLicenseSchemaWithResponse(context.Background(), tc.product.ProductID)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, read.StatusCode())
	})

	t.Run("the product can declare a fresh schema afterwards", func(t *testing.T) {
		tc := newTestCtx(t)
		client := seedSchema(t, tc)

		del, err := client.DeleteLicenseSchemaWithResponse(context.Background(), tc.product.ProductID)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, del.StatusCode(), string(del.Body))

		recreated, err := client.CreateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseSchemaJSONRequestBody{
				Fields: []ct.LicenseFieldDeclaration{
					itdsl.LicenseField("seats", ct.LicenseFieldTypeLIMIT, nil),
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, recreated.StatusCode(), string(recreated.Body))
		require.NotNil(t, recreated.JSON201)
		require.Len(t, recreated.JSON201.Fields, 1)
		assert.Equal(t, "seats", recreated.JSON201.Fields[0].Name)
	})

	t.Run("404 when the product has declared no schema", func(t *testing.T) {
		tc := newTestCtx(t)

		resp, err := tc.product.OwnerAuthenticatedClient().DeleteLicenseSchemaWithResponse(
			context.Background(), tc.product.ProductID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

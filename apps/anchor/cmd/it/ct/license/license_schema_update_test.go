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

// seedSchema declares a two-field schema and returns the product's client.
func seedSchema(t *testing.T, tc testCtx) *ct.ClientWithResponses {
	t.Helper()
	itdsl.Given(t).
		ExistingProduct(itdsl.ExistingProductOpts{Alias: "p", Context: tc.product}).
		LicenseSchema(itdsl.LicenseSchemaOpts{
			Alias:        "schema",
			ProductAlias: "p",
			Description:  new("original"),
			Fields: []ct.LicenseFieldDeclaration{
				itdsl.LicenseField("flows", ct.LicenseFieldTypeLIMIT, limitRules(0, 500)),
				itdsl.LicenseField("sso", ct.LicenseFieldTypeBOOLEAN, nil),
			},
		}).
		Build()
	return tc.product.OwnerAuthenticatedClient()
}

func TestLicenseSchemaUpdate(t *testing.T) {
	t.Run("replaces the field declaration wholesale", func(t *testing.T) {
		tc := newTestCtx(t)
		client := seedSchema(t, tc)

		resp, err := client.UpdateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.UpdateLicenseSchemaJSONRequestBody{
				Fields: &[]ct.LicenseFieldDeclaration{
					itdsl.LicenseField("flows", ct.LicenseFieldTypeLIMIT, limitRules(0, 5000)),
					itdsl.LicenseField("seats", ct.LicenseFieldTypeLIMIT, nil),
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		require.Len(t, resp.JSON200.Fields, 2)
		assert.Equal(t, "flows", resp.JSON200.Fields[0].Name)
		require.NotNil(t, resp.JSON200.Fields[0].Rules.Max)
		assert.InDelta(t, 5000.0, *resp.JSON200.Fields[0].Rules.Max, 0)
		// `sso` was absent from the request, so it is gone: a schema is one
		// declaration, and an omitted field is a removal.
		assert.Equal(t, "seats", resp.JSON200.Fields[1].Name)

		read, err := client.GetLicenseSchemaWithResponse(context.Background(), tc.product.ProductID)
		require.NoError(t, err)
		require.NotNil(t, read.JSON200)
		require.Len(t, read.JSON200.Fields, 2)
		assert.Equal(t, "seats", read.JSON200.Fields[1].Name)
	})

	t.Run("leaves fields alone when the request omits them", func(t *testing.T) {
		tc := newTestCtx(t)
		client := seedSchema(t, tc)

		resp, err := client.UpdateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.UpdateLicenseSchemaJSONRequestBody{Description: new("edited")},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		require.NotNil(t, resp.JSON200.Description)
		assert.Equal(t, "edited", *resp.JSON200.Description)
		require.Len(t, resp.JSON200.Fields, 2)
		assert.Equal(t, "flows", resp.JSON200.Fields[0].Name)
		assert.Equal(t, "sso", resp.JSON200.Fields[1].Name)
	})

	t.Run("clears the declaration when handed an empty list", func(t *testing.T) {
		tc := newTestCtx(t)
		client := seedSchema(t, tc)

		resp, err := client.UpdateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.UpdateLicenseSchemaJSONRequestBody{Fields: &[]ct.LicenseFieldDeclaration{}},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)
		assert.Empty(t, resp.JSON200.Fields)
	})

	t.Run("404 when the product has declared no schema", func(t *testing.T) {
		tc := newTestCtx(t)

		resp, err := tc.product.OwnerAuthenticatedClient().UpdateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.UpdateLicenseSchemaJSONRequestBody{Description: new("nothing to edit")},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})

	t.Run("a rejected update leaves the stored declaration untouched", func(t *testing.T) {
		tc := newTestCtx(t)
		client := seedSchema(t, tc)

		resp, err := client.UpdateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.UpdateLicenseSchemaJSONRequestBody{
				Fields: &[]ct.LicenseFieldDeclaration{
					itdsl.LicenseField("seats", ct.LicenseFieldTypeLIMIT, limitRules(100, 10)),
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))

		read, err := client.GetLicenseSchemaWithResponse(context.Background(), tc.product.ProductID)
		require.NoError(t, err)
		require.NotNil(t, read.JSON200)
		require.Len(t, read.JSON200.Fields, 2)
		assert.Equal(t, "flows", read.JSON200.Fields[0].Name)
		assert.Equal(t, "sso", read.JSON200.Fields[1].Name)
	})
}

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

func TestLicenseSchemaGet(t *testing.T) {
	t.Run("reads back the declaration", func(t *testing.T) {
		tc := newTestCtx(t)
		state := itdsl.Given(t).
			ExistingProduct(itdsl.ExistingProductOpts{Alias: "p", Context: tc.product}).
			LicenseSchema(itdsl.LicenseSchemaOpts{
				Alias:        "schema",
				ProductAlias: "p",
				Description:  new("Pro tier surface"),
				Fields: []ct.LicenseFieldDeclaration{
					itdsl.LicenseField("flows", ct.LicenseFieldTypeLIMIT, limitRules(0, 500), new(ct.GAUGE)),
					itdsl.LicenseField("sso", ct.LicenseFieldTypeBOOLEAN, nil, nil),
				},
			}).
			Build()

		resp, err := tc.product.OwnerAuthenticatedClient().GetLicenseSchemaWithResponse(
			context.Background(), tc.product.ProductID,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON200)

		assert.Equal(t, state.LicenseSchema("schema").ID, resp.JSON200.Id)
		require.NotNil(t, resp.JSON200.Description)
		assert.Equal(t, "Pro tier surface", *resp.JSON200.Description)
		require.Len(t, resp.JSON200.Fields, 2)
		assert.Equal(t, "flows", resp.JSON200.Fields[0].Name)
		require.NotNil(t, resp.JSON200.Fields[0].Rules.Max)
		assert.InDelta(t, 500.0, *resp.JSON200.Fields[0].Rules.Max, 0)
		require.NotNil(t, resp.JSON200.Fields[0].UsageShape)
		assert.Equal(t, ct.GAUGE, *resp.JSON200.Fields[0].UsageShape)
		assert.Equal(t, "sso", resp.JSON200.Fields[1].Name)
		assert.Nil(t, resp.JSON200.Fields[1].UsageShape)
	})

	t.Run("404 when the product has declared no schema", func(t *testing.T) {
		tc := newTestCtx(t)

		resp, err := tc.product.OwnerAuthenticatedClient().GetLicenseSchemaWithResponse(
			context.Background(), tc.product.ProductID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

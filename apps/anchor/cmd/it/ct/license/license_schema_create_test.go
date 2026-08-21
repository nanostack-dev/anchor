package license_ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseSchemaCreate(t *testing.T) {
	t.Run("declares every field type", func(t *testing.T) {
		tc := newTestCtx(t)
		client := tc.product.OwnerAuthenticatedClient()

		resp, err := client.CreateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseSchemaJSONRequestBody{
				Description: new("Billing-facing declaration"),
				Fields: []ct.LicenseFieldDeclaration{
					{
						Name:       "flows",
						Type:       ct.LicenseFieldTypeLIMIT,
						Rules:      limitRules(0, 100000),
						UsageShape: new(ct.GAUGE),
					},
					{Name: "burst_credit", Type: ct.LicenseFieldTypeNUMBER},
					{Name: "sso", Type: ct.LicenseFieldTypeBOOLEAN},
					{
						Name:  "support_tier",
						Type:  ct.LicenseFieldTypeENUM,
						Rules: &ct.LicenseFieldRules{Values: enumValues("basic", "priority")},
					},
					{
						Name:  "region",
						Type:  ct.LicenseFieldTypeSTRING,
						Rules: &ct.LicenseFieldRules{Pattern: new("^[a-z]{2}-[a-z]+$"), MaxLength: new(32)},
					},
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON201)

		schema := resp.JSON201
		assert.NotEmpty(t, schema.Id)
		assert.Equal(t, tc.product.ProductID, schema.ProductId)
		require.NotNil(t, schema.Description)
		assert.Equal(t, "Billing-facing declaration", *schema.Description)
		require.Len(t, schema.Fields, 5)

		// Fields read back ordered by name, whatever order they were declared in.
		assert.Equal(
			t,
			[]string{"burst_credit", "flows", "region", "sso", "support_tier"},
			fieldNames(schema.Fields),
		)
		for _, f := range schema.Fields {
			assert.NotEmpty(t, f.Id)
		}

		flows := fieldByName(t, schema.Fields, "flows")
		assert.Equal(t, ct.LicenseFieldTypeLIMIT, flows.Type)
		require.NotNil(t, flows.Rules.Min)
		require.NotNil(t, flows.Rules.Max)
		assert.InDelta(t, 0.0, *flows.Rules.Min, 0)
		assert.InDelta(t, 100000.0, *flows.Rules.Max, 0)
		require.NotNil(t, flows.UsageShape)
		assert.Equal(t, ct.GAUGE, *flows.UsageShape)

		// A field declared without rules reads back as an empty rule set, not a
		// missing one.
		burst := fieldByName(t, schema.Fields, "burst_credit")
		assert.Nil(t, burst.Rules.Min)
		assert.Nil(t, burst.Rules.Max)

		tier := fieldByName(t, schema.Fields, "support_tier")
		require.NotNil(t, tier.Rules.Values)
		assert.Equal(t, []string{"basic", "priority"}, *tier.Rules.Values)

		region := fieldByName(t, schema.Fields, "region")
		require.NotNil(t, region.Rules.Pattern)
		assert.Equal(t, "^[a-z]{2}-[a-z]+$", *region.Rules.Pattern)
	})

	t.Run("accepts a schema with no fields", func(t *testing.T) {
		tc := newTestCtx(t)
		client := tc.product.OwnerAuthenticatedClient()

		resp, err := client.CreateLicenseSchemaWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseSchemaJSONRequestBody{Fields: []ct.LicenseFieldDeclaration{}},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON201)
		assert.Empty(t, resp.JSON201.Fields)
	})

	t.Run("rejects a second schema for the same product", func(t *testing.T) {
		tc := newTestCtx(t)
		client := tc.product.OwnerAuthenticatedClient()
		body := ct.CreateLicenseSchemaJSONRequestBody{
			Fields: []ct.LicenseFieldDeclaration{
				{Name: "flows", Type: ct.LicenseFieldTypeLIMIT, UsageShape: new(ct.GAUGE)},
			},
		}

		first, err := client.CreateLicenseSchemaWithResponse(context.Background(), tc.product.ProductID, body)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, first.StatusCode(), string(first.Body))

		second, err := client.CreateLicenseSchemaWithResponse(context.Background(), tc.product.ProductID, body)
		require.NoError(t, err)
		// Current state (a schema already declared) refuses the request, and
		// deleting it is the later request that lets a retry succeed, so this
		// is a 409, not a 400. The generated client has no typed 409 getter
		// for this route yet, so the body is decoded by hand.
		require.Equal(t, http.StatusConflict, second.StatusCode(), string(second.Body))
		var errResp ct.ApiErrorResponse
		require.NoError(t, json.Unmarshal(second.Body, &errResp))
		assertAPIError(t, errResp.Errors, "LICENSE_SCHEMA_EXISTS")
	})

	t.Run("schemas are scoped to their own product", func(t *testing.T) {
		first := newTestCtx(t)
		second := newTestCtx(t)

		firstResp, err := first.product.OwnerAuthenticatedClient().CreateLicenseSchemaWithResponse(
			context.Background(),
			first.product.ProductID,
			ct.CreateLicenseSchemaJSONRequestBody{
				Fields: []ct.LicenseFieldDeclaration{
					{Name: "flows", Type: ct.LicenseFieldTypeLIMIT, UsageShape: new(ct.GAUGE)},
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, firstResp.StatusCode(), string(firstResp.Body))

		// The same field name in a different product is a different declaration,
		// not a collision.
		secondResp, err := second.product.OwnerAuthenticatedClient().CreateLicenseSchemaWithResponse(
			context.Background(),
			second.product.ProductID,
			ct.CreateLicenseSchemaJSONRequestBody{
				Fields: []ct.LicenseFieldDeclaration{
					{Name: "flows", Type: ct.LicenseFieldTypeLIMIT, UsageShape: new(ct.GAUGE)},
				},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, secondResp.StatusCode(), string(secondResp.Body))

		read, err := second.product.OwnerAuthenticatedClient().GetLicenseSchemaWithResponse(
			context.Background(), second.product.ProductID,
		)
		require.NoError(t, err)
		require.NotNil(t, read.JSON200)
		assert.NotEqual(t, firstResp.JSON201.Id, read.JSON200.Id)
	})
}

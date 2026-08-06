package license_ct_test

import (
	"context"
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
						Name:     "flows",
						Type:     ct.LicenseFieldTypeLimit,
						Required: new(true),
						Rules:    limitRules(0, 100000),
					},
					{Name: "burst_credit", Type: ct.LicenseFieldTypeNumber},
					{Name: "sso", Type: ct.LicenseFieldTypeBoolean, Required: new(true)},
					{
						Name:  "support_tier",
						Type:  ct.LicenseFieldTypeEnum,
						Rules: &ct.LicenseFieldRules{Values: enumValues("basic", "priority")},
					},
					{
						Name:  "region",
						Type:  ct.LicenseFieldTypeString,
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

		// Declaration order is preserved, so a rendered form reads the way its
		// author wrote it.
		names := make([]string, 0, len(schema.Fields))
		for _, f := range schema.Fields {
			names = append(names, f.Name)
			assert.NotEmpty(t, f.Id)
		}
		assert.Equal(t, []string{"flows", "burst_credit", "sso", "support_tier", "region"}, names)

		flows := schema.Fields[0]
		assert.Equal(t, ct.LicenseFieldTypeLimit, flows.Type)
		assert.True(t, flows.Required)
		require.NotNil(t, flows.Rules.Min)
		require.NotNil(t, flows.Rules.Max)
		assert.InDelta(t, 0.0, *flows.Rules.Min, 0)
		assert.InDelta(t, 100000.0, *flows.Rules.Max, 0)

		// A field declared without rules reads back as an empty rule set, not a
		// missing one.
		burst := schema.Fields[1]
		assert.False(t, burst.Required)
		assert.Nil(t, burst.Rules.Min)
		assert.Nil(t, burst.Rules.Max)

		require.NotNil(t, schema.Fields[3].Rules.Values)
		assert.Equal(t, []string{"basic", "priority"}, *schema.Fields[3].Rules.Values)
		require.NotNil(t, schema.Fields[4].Rules.Pattern)
		assert.Equal(t, "^[a-z]{2}-[a-z]+$", *schema.Fields[4].Rules.Pattern)
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
			Fields: []ct.LicenseFieldDeclaration{{Name: "flows", Type: ct.LicenseFieldTypeLimit}},
		}

		first, err := client.CreateLicenseSchemaWithResponse(context.Background(), tc.product.ProductID, body)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, first.StatusCode(), string(first.Body))

		second, err := client.CreateLicenseSchemaWithResponse(context.Background(), tc.product.ProductID, body)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, second.StatusCode(), string(second.Body))
		require.NotNil(t, second.JSON409)
		assertAPIError(t, second.JSON409.Errors, "LICENSE_SCHEMA_EXISTS")
	})

	t.Run("schemas are scoped to their own product", func(t *testing.T) {
		first := newTestCtx(t)
		second := newTestCtx(t)

		firstResp, err := first.product.OwnerAuthenticatedClient().CreateLicenseSchemaWithResponse(
			context.Background(),
			first.product.ProductID,
			ct.CreateLicenseSchemaJSONRequestBody{
				Fields: []ct.LicenseFieldDeclaration{{Name: "flows", Type: ct.LicenseFieldTypeLimit}},
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
				Fields: []ct.LicenseFieldDeclaration{{Name: "flows", Type: ct.LicenseFieldTypeLimit}},
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

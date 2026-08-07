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

func TestLicenseTemplateCreate(t *testing.T) {
	t.Run("stores a named set of values", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := tc.product.OwnerAuthenticatedClient().CreateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseTemplateJSONRequestBody{
				Name:        "Pro",
				Description: new("The paid tier"),
				Values:      validTemplateValues(),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON201)

		template := resp.JSON201
		assert.NotEmpty(t, template.Id)
		assert.Equal(t, tc.product.ProductID, template.ProductId)
		assert.Equal(t, "Pro", template.Name)
		require.NotNil(t, template.Description)
		assert.Equal(t, "The paid tier", *template.Description)

		// A number survives the round-trip through JSONB as a number, not as the
		// string a naive storage layer would hand back.
		assert.InDelta(t, 500.0, template.Values["flows"], 0)
		assert.Equal(t, true, template.Values["sso"])
		assert.Equal(t, "priority", template.Values["support_tier"])
		assert.Equal(t, "ca-central", template.Values["region"])
	})

	t.Run("carries a value for every declared field", func(t *testing.T) {
		tc := newTemplateCtx(t)

		template := createTemplate(t, tc, "Free", templateValuesWith("flows", 10))

		// The whole point of the mandatory rule: reading a template answers what
		// the customer has, for every field, without anyone deciding what an
		// absent one would have meant.
		assert.Len(t, template.Values, len(templateSchemaFields()))
		for _, declared := range templateSchemaFields() {
			assert.Contains(t, template.Values, declared.Name)
		}
	})

	t.Run("carries no version or lifecycle state", func(t *testing.T) {
		tc := newTemplateCtx(t)

		resp, err := tc.product.OwnerAuthenticatedClient().CreateLicenseTemplateWithResponse(
			context.Background(),
			tc.product.ProductID,
			ct.CreateLicenseTemplateJSONRequestBody{
				Name:   uniqueTemplateName(),
				Values: validTemplateValues(),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))

		// Asserted on the wire rather than on the generated struct, because the
		// struct can only ever have the fields the contract declares. A template
		// is consulted once, at instantiation, so there is nothing to publish and
		// nothing to archive — and adding either later would be a design change,
		// not an addition.
		var body map[string]any
		require.NoError(t, json.Unmarshal(resp.Body, &body))
		for _, absent := range []string{"version", "status", "state", "lifecycle", "published_at"} {
			assert.NotContains(t, body, absent)
		}
	})

	t.Run("templates are scoped to their own product", func(t *testing.T) {
		first := newTemplateCtx(t)
		second := newTemplateCtx(t)

		// The same name in a different product is a different template, not a
		// collision.
		firstTemplate := createTemplate(t, first, "Pro", validTemplateValues())
		secondTemplate := createTemplate(t, second, "Pro", validTemplateValues())
		assert.NotEqual(t, firstTemplate.Id, secondTemplate.Id)

		// And one product cannot read the other's by identifier.
		crossRead, err := second.product.OwnerAuthenticatedClient().GetLicenseTemplateWithResponse(
			context.Background(), second.product.ProductID, firstTemplate.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, crossRead.StatusCode(), string(crossRead.Body))
	})
}

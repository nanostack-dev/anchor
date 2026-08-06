package ct_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductOrganizationMetadata(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	apiKeyClient, _ := testProduct.CreateAPIKeyClientWithAllScopes()

	t.Run(
		"MetadataSurvivesCreateAndGet", func(t *testing.T) {
			metadata := ct.Metadata{
				"billing_ref": "cust_abc123",
				"region":      "us-east-1",
				"sla_level":   "gold",
			}

			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:     "Metadata Create Org",
					Metadata: &metadata,
				},
			)

			require.NoError(t, err, "create organization should not error")
			require.Equal(t, http.StatusCreated, createResponse.StatusCode())
			require.NotNil(t, createResponse.JSON201)
			require.NotNil(
				t, createResponse.JSON201.Metadata,
				"create response should echo the stored metadata",
			)
			assert.Equal(t, metadata, *createResponse.JSON201.Metadata)

			getResponse, err := apiKeyClient.GetProductOrganizationWithResponse(
				ctx, testProduct.ProductID, createResponse.JSON201.Id,
			)

			require.NoError(t, err, "get organization should not error")
			require.NotNil(t, getResponse.JSON200)
			require.NotNil(
				t, getResponse.JSON200.Metadata,
				"metadata should be persisted, not discarded",
			)
			assert.Equal(t, metadata, *getResponse.JSON200.Metadata)
		},
	)

	t.Run(
		"MetadataIsAbsentWhenNotProvided", func(t *testing.T) {
			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{Name: "No Metadata Org"},
			)

			require.NoError(t, err)
			require.NotNil(t, createResponse.JSON201)
			assert.Nil(t, createResponse.JSON201.Metadata, "metadata should stay unset")
		},
	)

	t.Run(
		"UpdateReplacesMetadataInFull", func(t *testing.T) {
			initial := ct.Metadata{"billing_ref": "cust_abc123", "region": "us-east-1"}

			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:     "Metadata Update Org",
					Metadata: &initial,
				},
			)
			require.NoError(t, err)
			require.NotNil(t, createResponse.JSON201)

			replacement := ct.Metadata{"sla_level": "gold"}
			updateResponse, err := apiKeyClient.UpdateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				createResponse.JSON201.Id,
				ct.UpdateProductOrganizationJSONRequestBody{
					Name:     "Metadata Update Org",
					Metadata: &replacement,
				},
			)

			require.NoError(t, err)
			require.NotNil(t, updateResponse.JSON200)
			require.NotNil(t, updateResponse.JSON200.Metadata)
			assert.Equal(
				t, replacement, *updateResponse.JSON200.Metadata,
				"update should replace metadata wholesale, not merge",
			)
		},
	)

	t.Run(
		"UpdateWithoutMetadataClearsIt", func(t *testing.T) {
			initial := ct.Metadata{"billing_ref": "cust_abc123"}

			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:     "Metadata Clear Org",
					Metadata: &initial,
				},
			)
			require.NoError(t, err)
			require.NotNil(t, createResponse.JSON201)

			updateResponse, err := apiKeyClient.UpdateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				createResponse.JSON201.Id,
				ct.UpdateProductOrganizationJSONRequestBody{Name: "Metadata Clear Org"},
			)

			require.NoError(t, err)
			require.NotNil(t, updateResponse.JSON200)
			assert.Nil(
				t, updateResponse.JSON200.Metadata,
				"omitting metadata on PUT clears it, matching description semantics",
			)
		},
	)

	t.Run(
		"RejectsNonScalarMetadataValue", func(t *testing.T) {
			metadata := ct.Metadata{"billing": map[string]any{"ref": "cust_abc123"}}

			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:     "Invalid Metadata Org",
					Metadata: &metadata,
				},
			)

			require.NoError(t, err, "request should complete and be rejected by the API")
			assert.Equal(t, http.StatusBadRequest, createResponse.StatusCode())
		},
	)

	t.Run(
		"RejectsOverlongMetadataValue", func(t *testing.T) {
			metadata := ct.Metadata{"note": strings.Repeat("v", 513)}

			createResponse, err := apiKeyClient.CreateProductOrganizationWithResponse(
				ctx,
				testProduct.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:     "Overlong Metadata Org",
					Metadata: &metadata,
				},
			)

			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, createResponse.StatusCode())
		},
	)

	t.Run(
		"MetadataIsExposedOnUserOrganization", func(t *testing.T) {
			productCtx := createTestProductContext(t)
			productUser := createDSLProductUser(t, productCtx)
			orgClient, _ := productCtx.CreateAPIKeyClientWithAllScopes()

			metadata := ct.Metadata{"billing_ref": "cust_abc123"}
			orgResponse, err := orgClient.CreateProductOrganizationWithResponse(
				ctx,
				productCtx.ProductID,
				ct.CreateProductOrganizationJSONRequestBody{
					Name:     "User View Metadata Org",
					Metadata: &metadata,
				},
			)
			require.NoError(t, err)
			require.NotNil(t, orgResponse.JSON201)

			role := createDSLProductRole(t, productCtx, "Developer", new("Developer role"))
			createDSLMembership(t, productCtx, productUser.ID, orgResponse.JSON201.Id, role.ID)

			readClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
			getResponse, err := readClient.GetUserOrganizationWithResponse(
				ctx, productCtx.ProductID, productUser.ID, orgResponse.JSON201.Id, nil,
			)

			require.NoError(t, err)
			require.NotNil(t, getResponse.JSON200)
			require.NotNil(
				t, getResponse.JSON200.Organization.Metadata,
				"user-facing organization view should expose metadata too",
			)
			assert.Equal(t, metadata, *getResponse.JSON200.Organization.Metadata)
		},
	)
}

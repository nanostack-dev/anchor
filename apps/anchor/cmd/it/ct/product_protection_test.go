package ct_test

import (
	"net/http"
	"testing"

	itdsl "anchor/cmd/it/shared/dsl"

	client "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductProtection(t *testing.T) {
	t.Parallel()
	product := createTestProductContext(t)
	management := product.Management()
	initial := management.Get()
	assert.False(t, initial.Config.Protected)

	organization := product.Organizations().Create(client.CreateProductOrganizationJSONRequestBody{
		Name: "Protected organization " + ids.MustNew("test"),
	})
	protected := management.Update(client.UpdateProductJSONRequestBody{
		Name:   initial.Name,
		Config: &client.ProductConfigRequest{Protected: new(true)},
	})
	assert.True(t, protected.Config.Protected)
	assert.True(t, management.Get().Config.Protected)

	refused := management.DeleteRaw()
	require.Equal(t, http.StatusConflict, refused.StatusCode(), string(refused.Body))
	require.NotNil(t, refused.JSON409)
	require.NotEmpty(t, refused.JSON409.Errors)
	assert.Equal(t, "PRODUCT_PROTECTED", refused.JSON409.Errors[0].Code)
	assert.Equal(t, organization.Id, product.Organizations().Get(organization.Id).Id)

	withoutConfig := management.Update(client.UpdateProductJSONRequestBody{
		Name:        initial.Name,
		Description: new("Still protected after editing details"),
	})
	assert.True(t, withoutConfig.Config.Protected)

	withoutProtection := management.Update(client.UpdateProductJSONRequestBody{
		Name: initial.Name,
		Config: &client.ProductConfigRequest{
			OrganizationApiKeys: &client.ProductOrganizationAPIKeysConfigRequest{Prefix: "protected"},
		},
	})
	assert.True(t, withoutProtection.Config.Protected)
	assert.True(t, management.Get().Config.Protected)
	require.Equal(t, http.StatusConflict, management.DeleteRaw().StatusCode())

	unprotected := management.Update(client.UpdateProductJSONRequestBody{
		Name:   initial.Name,
		Config: &client.ProductConfigRequest{Protected: new(false)},
	})
	assert.False(t, unprotected.Config.Protected)
	management.Delete()
	assert.Equal(t, http.StatusNotFound, management.GetRaw().StatusCode())
}

func TestCreateProtectedProduct(t *testing.T) {
	t.Parallel()
	product := createTestProductContext(t)
	created := product.Management().Create(client.CreateProductJSONRequestBody{
		Name:   "Protected product " + ids.MustNew("test"),
		Config: &client.ProductConfigRequest{Protected: new(true)},
	})
	assert.True(t, created.Config.Protected)
	product.ProductID = created.Id
	assert.True(t, product.Management().Get().Config.Protected)
	refused := product.Management().DeleteRaw()
	require.Equal(t, http.StatusConflict, refused.StatusCode(), string(refused.Body))
	require.NotNil(t, refused.JSON409)
	assert.Equal(t, "PRODUCT_PROTECTED", refused.JSON409.Errors[0].Code)
}

func TestProtectedProductTenantBoundary(t *testing.T) {
	t.Parallel()
	product := createTestProductContext(t)
	management := product.Management()
	initial := management.Get()
	management.Update(client.UpdateProductJSONRequestBody{
		Name:   initial.Name,
		Config: &client.ProductConfigRequest{Protected: new(true)},
	})

	otherTenant := itdsl.Given(t).Tenant(itdsl.TenantOpts{Alias: "other", Isolated: true}).Build().Tenant("other")
	foreign := management.As(otherTenant.OwnerClient)
	assert.Equal(t, http.StatusNotFound, foreign.DeleteRaw().StatusCode())
	assert.Equal(t, http.StatusNotFound, foreign.UpdateRaw(client.UpdateProductJSONRequestBody{
		Name:   initial.Name,
		Config: &client.ProductConfigRequest{Protected: new(false)},
	}).StatusCode())
	assert.True(t, management.Get().Config.Protected)
}

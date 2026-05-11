package itdsl

import (
	"github.com/stretchr/testify/require"

	dslfactory "anchor/cmd/it/shared/dsl/factory"
)

// ProductOpts configures product context creation in the fluent builder.
type ProductOpts struct {
	Alias       string
	TenantAlias string
}

// Product creates a product context under a tenant alias.
func (b *Builder) Product(opts ProductOpts) *Builder {
	b.t.Helper()

	require.NotEmpty(b.t, opts.Alias, "product alias is required")
	require.NotEmpty(b.t, opts.TenantAlias, "tenant alias is required")

	_, exists := b.products[opts.Alias]
	require.False(b.t, exists, "product alias '%s' already exists", opts.Alias)

	tenantCtx, tenantExists := b.tenants[opts.TenantAlias]
	require.True(b.t, tenantExists, "unknown tenant alias '%s'", opts.TenantAlias)

	createdProduct := dslfactory.CreateProductWithDefaultPermissions(b.t, tenantCtx.ID)
	productCtx := &ProductContext{
		ProductID:                createdProduct.ID,
		ServerURL:                tenantCtx.ServerURL,
		testingContext:           b.t,
		ownerAuthenticatedClient: tenantCtx.OwnerClient,
	}
	productCtx.allScopeAPIKeyClient, productCtx.AllScopeAPIKey = productCtx.CreateAPIKeyClientWithAllScopes()
	b.products[opts.Alias] = productCtx

	return b
}

package reconcile_test

import (
	"testing"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
)

func createTestProductContext(t *testing.T) *itdsl.ProductContext {
	t.Helper()
	tenantAlias := "tenant.product." + itshared.Faker.UUID().V4()
	productAlias := "product.context." + itshared.Faker.UUID().V4()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: tenantAlias}).
		Product(itdsl.ProductOpts{Alias: productAlias, TenantAlias: tenantAlias}).
		Build()
	return state.Product(productAlias)
}

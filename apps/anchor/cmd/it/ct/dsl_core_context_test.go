package ct_test

import (
	"sync"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
)

const defaultTenantAlias = "tenant.default"

var (
	defaultStateOnce sync.Once
	defaultState     *itdsl.State
)

func testState(t *testing.T) *itdsl.State {
	t.Helper()

	defaultStateOnce.Do(func() {
		defaultState = itdsl.Given(t).
			Tenant(itdsl.TenantOpts{Alias: defaultTenantAlias}).
			Build()
	})

	return defaultState
}

func testTenant(t *testing.T) *itdsl.Tenant {
	t.Helper()
	return testState(t).Tenant(defaultTenantAlias)
}

func testOwnerUser(t *testing.T) *itdsl.PlatformUser {
	t.Helper()

	tenant := testTenant(t)
	return tenant.OwnerUser
}

func testOwnerClient(t *testing.T) *nanostackClient.ClientWithResponses {
	t.Helper()

	return testTenant(t).OwnerClient
}

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

func createPlatformAdmin(t *testing.T) *itdsl.PlatformUser {
	t.Helper()

	return itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "tenant.local"}).
		PlatformAdmin(
			itdsl.PlatformAdminOpts{Alias: "platform.admin.local", TenantAlias: "tenant.local"},
		).
		Build().
		PlatformUser("platform.admin.local")
}

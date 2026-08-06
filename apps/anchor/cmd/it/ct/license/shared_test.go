// Package license_ct_test contains component tests for the license schema HTTP
// API. Every path goes through the real chi router and the oapi-codegen strict
// server, backed by a live Postgres, and is exercised through the generated
// public client — the same surface a consuming product uses.
package license_ct_test

import (
	"os"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
)

func TestMain(m *testing.M) {
	if err := os.Chdir(".."); err != nil {
		panic(err)
	}
	itshared.RunTestMain(m, itshared.TestConfig{
		EnableRedis:             true,
		PopulateRepositories:    true,
		APIKeyService:           &itshared.APIKeyService,
		PermissionRepository:    &itshared.PermissionRepository,
		ProductRepository:       &itshared.ProductRepository,
		ProductUserRepository:   &itshared.ProductUserRepository,
		OrgMembershipRepository: &itshared.OrgMembershipRepository,
		TenantRepository:        &itshared.TenantRepository,
		UserRepository:          &itshared.UserRepository,
		PlatformUserRepository:  &itshared.PlatformTenantUserRepo,
		JWTHelper:               &itshared.JWTHelper,
	})
}

type testCtx struct {
	tenantID string
	product  *itdsl.ProductContext
	state    *itdsl.State
}

// newTestCtx builds an isolated tenant and product. A license schema is a
// singleton on its product, so every test that writes one needs its own
// product rather than a shared fixture.
func newTestCtx(t *testing.T) testCtx {
	t.Helper()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true}).
		Product(itdsl.ProductOpts{Alias: "p", TenantAlias: "t"}).
		Build()
	return testCtx{
		tenantID: state.Tenant("t").ID,
		product:  state.Product("p"),
		state:    state,
	}
}

func uniqueFieldName() string {
	return "field_" + ids.MustNew("ct")
}

func assertAPIError(t *testing.T, errs []ct.ApiError, code string) {
	t.Helper()
	require.Len(t, errs, 1)
	assert.Equal(t, code, errs[0].Code)
}

// assertFieldError pins the structured shape a malformed declaration must
// produce: one detail, naming the offending license field and the rule it
// violated. A caller rendering a form has to be able to highlight the row
// without parsing prose.
func assertFieldError(t *testing.T, errs []ct.ApiError, code, field, rule string) {
	t.Helper()
	require.Len(t, errs, 1)
	assert.Equal(t, code, errs[0].Code)
	assert.Equal(t, field, deref(errs[0].Field))
	if rule == "" {
		return
	}
	require.NotNil(t, errs[0].Metadata)
	assert.Equal(t, rule, (*errs[0].Metadata)["rule"])
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func limitRules(minValue, maxValue float64) *ct.LicenseFieldRules {
	return &ct.LicenseFieldRules{Min: &minValue, Max: &maxValue}
}

// fieldByName looks a declared field up by name. Fields are read back ordered
// by name, so asserting through this rather than an index keeps a test about
// one field's rules from breaking when a neighbour is renamed.
func fieldByName(t *testing.T, fields []ct.LicenseFieldResponse, name string) ct.LicenseFieldResponse {
	t.Helper()
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	require.FailNowf(t, "field not declared", "no field named %q in the schema", name)
	return ct.LicenseFieldResponse{}
}

func fieldNames(fields []ct.LicenseFieldResponse) []string {
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Name)
	}
	return names
}

// enumValues builds the allowed-value list an enum field draws from. The
// generated client models an optional array as a pointer, so an empty list has
// to stay distinguishable from an absent one — that difference is exactly what
// the "enum with no allowed values" case exercises.
func enumValues(values ...string) *[]string {
	if values == nil {
		values = []string{}
	}
	return &values
}

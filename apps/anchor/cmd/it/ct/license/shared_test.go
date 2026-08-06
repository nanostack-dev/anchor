// Package license_ct_test contains component tests for the licensing HTTP API:
// the per-product license schema and the license templates declared against it.
// Every path goes through the real chi router and the oapi-codegen strict
// server, backed by a live Postgres, and is exercised through the generated
// public client — the same surface a consuming product uses.
package license_ct_test

import (
	"context"
	"net/http"
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

// templateSchemaFields is the declaration the license template tests are
// written against: one required limit with a ceiling, one required boolean, one
// optional enum, and one optional pattern-constrained string. Between them they
// reach every branch a template write has to take — required and optional,
// bounded and unbounded, numeric and not.
func templateSchemaFields() []ct.LicenseFieldDeclaration {
	return []ct.LicenseFieldDeclaration{
		{
			Name:     "flows",
			Type:     ct.LicenseFieldTypeLIMIT,
			Required: new(true),
			Rules:    limitRules(0, 100000),
		},
		{Name: "sso", Type: ct.LicenseFieldTypeBOOLEAN, Required: new(true)},
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
	}
}

// declareSchema gives the product a license schema, since a template is defined
// as a set of values satisfying one and cannot be written without it.
func declareSchema(t *testing.T, tc testCtx, fields []ct.LicenseFieldDeclaration) {
	t.Helper()
	resp, err := tc.product.OwnerAuthenticatedClient().CreateLicenseSchemaWithResponse(
		context.Background(),
		tc.product.ProductID,
		ct.CreateLicenseSchemaJSONRequestBody{Fields: fields},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
}

// newTemplateCtx builds an isolated product that already declares the schema
// the template tests are written against.
func newTemplateCtx(t *testing.T) testCtx {
	t.Helper()
	tc := newTestCtx(t)
	declareSchema(t, tc, templateSchemaFields())
	return tc
}

// validTemplateValues satisfies templateSchemaFields: both required fields set,
// every value inside its declared rules.
func validTemplateValues() ct.LicenseTemplateValues {
	return ct.LicenseTemplateValues{
		"flows":        500,
		"sso":          true,
		"support_tier": "priority",
	}
}

// createTemplate writes a template and returns it, failing the test if the
// write is refused. Tests about reading, listing or editing use it so their
// setup cannot be mistaken for the behaviour under test.
func createTemplate(
	t *testing.T, tc testCtx, name string, values ct.LicenseTemplateValues,
) ct.LicenseTemplateResponse {
	t.Helper()
	resp, err := tc.product.OwnerAuthenticatedClient().CreateLicenseTemplateWithResponse(
		context.Background(),
		tc.product.ProductID,
		ct.CreateLicenseTemplateJSONRequestBody{Name: name, Values: values},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON201)
	return *resp.JSON201
}

// uniqueTemplateName keeps two tests in the same package from colliding on the
// product-unique name, without making the name itself the subject of a test.
func uniqueTemplateName() string {
	return "tier_" + ids.MustNew("ct")
}

// missingTemplateID is a well-formed identifier for a template that was never
// written. It has to be a real KSUID: request validation rejects a malformed
// path parameter with a 400 before any handler runs, which would prove the
// contract's pattern rather than the handler's answer to "no such template".
func missingTemplateID() string {
	return ids.MustNew("ltpl")
}

func templateNames(templates []ct.LicenseTemplateResponse) []string {
	names := make([]string, 0, len(templates))
	for _, template := range templates {
		names = append(names, template.Name)
	}
	return names
}

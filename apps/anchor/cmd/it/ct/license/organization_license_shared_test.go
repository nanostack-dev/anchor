package license_ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// licenseCtx is a product that declares the schema the template tests are
// written against, plus an organization to stamp a template onto.
type licenseCtx struct {
	testCtx
	organizationID string
}

func newOrganizationLicenseCtx(t *testing.T) licenseCtx {
	t.Helper()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true}).
		Product(itdsl.ProductOpts{Alias: "p", TenantAlias: "t"}).
		ProductOrganization(itdsl.ProductOrganizationOpts{Alias: "o", ProductAlias: "p"}).
		Build()
	tc := testCtx{
		tenantID: state.Tenant("t").ID,
		product:  state.Product("p"),
		state:    state,
	}
	declareSchema(t, tc, templateSchemaFields())
	return licenseCtx{testCtx: tc, organizationID: state.ProductOrganization("o").ID}
}

// newOrganization adds a second organization to an existing product, for the
// tests whose subject is that one organization's license is its own.
func newOrganization(t *testing.T, lc licenseCtx) string {
	t.Helper()
	// Creating an organization is a Product API key route, not a platform bearer
	// one, so this cannot go through the owner client the licensing calls use.
	client, _ := lc.product.CreateAPIKeyClientWithScopes([]string{"organization:create"})
	resp, err := client.CreateProductOrganizationWithResponse(
		context.Background(),
		lc.product.ProductID,
		ct.CreateProductOrganizationJSONRequestBody{Name: "org_" + ids.MustNew("ct")},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON201)
	return resp.JSON201.Id
}

// instantiateLicense stamps a template onto the organization and returns the
// license, failing the test if the write is refused. Tests about reading,
// adjusting or diffing use it so their setup cannot be mistaken for the
// behaviour under test.
func instantiateLicense(
	t *testing.T, lc licenseCtx, templateID string,
) ct.OrganizationLicenseResponse {
	t.Helper()
	resp, err := lc.product.OwnerAuthenticatedClient().InstantiateOrganizationLicenseWithResponse(
		context.Background(),
		lc.product.ProductID,
		lc.organizationID,
		ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: templateID},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON201)
	return *resp.JSON201
}

// licensedOrganization is the common setup: a product with a schema, a template
// on it, and an organization already stamped from that template.
func licensedOrganization(t *testing.T) (licenseCtx, ct.LicenseTemplateResponse) {
	t.Helper()
	lc := newOrganizationLicenseCtx(t)
	template := createTemplate(t, lc.testCtx, uniqueTemplateName(), validTemplateValues())
	instantiateLicense(t, lc, template.Id)
	return lc, template
}

// instantiatedAt reads back when the copy was taken, for the tests whose
// subject is that a later write leaves the provenance alone.
func instantiatedAt(t *testing.T, lc licenseCtx) time.Time {
	t.Helper()
	resp, err := lc.product.OwnerAuthenticatedClient().GetOrganizationLicenseWithResponse(
		context.Background(), lc.product.ProductID, lc.organizationID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON200)
	return resp.JSON200.InstantiatedAt
}

// missingOrganizationID is a well-formed identifier for an organization that
// was never created. It has to be a real KSUID: request validation rejects a
// malformed path parameter with a 400 before any handler runs, which would
// prove the contract's pattern rather than the handler's answer.
func missingOrganizationID() string {
	return ids.MustNew("org")
}

// differenceByField looks a reported difference up by license field name.
// Differences come back ordered by name, so asserting through this rather than
// an index keeps a test about one field from breaking when a neighbour moves.
func differenceByField(
	t *testing.T, differences []ct.LicenseFieldDifference, field string,
) ct.LicenseFieldDifference {
	t.Helper()
	for _, difference := range differences {
		if difference.Field == field {
			return difference
		}
	}
	require.FailNowf(t, "field not in the diff", "no difference reported for %q", field)
	return ct.LicenseFieldDifference{}
}

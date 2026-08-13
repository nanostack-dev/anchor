package license_ct_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// The organization license tests drive a licenseWorld rather than raw clients.
// Each act method carries the require.NoError + status + NotNil triplet once,
// so a test body states what it does and what it expects, and nothing else.
// Every act has a *Raw twin returning the untouched response, for the tests
// whose subject is a refusal.

const (
	worldOrganizationAlias = "o"
	worldTemplateAlias     = "tpl"
)

// licenseWorld is a product declaring the schema in templateSchemaFields, with
// one organization and one template on it.
type licenseWorld struct {
	t        *testing.T
	state    *itdsl.State
	product  *itdsl.ProductContext
	tenantID string
}

// newLicenseWorld builds the product, schema, organization and template. The
// organization holds no license yet.
func newLicenseWorld(t *testing.T) *licenseWorld {
	t.Helper()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true}).
		Product(itdsl.ProductOpts{Alias: "p", TenantAlias: "t"}).
		LicenseSchema(itdsl.LicenseSchemaOpts{
			Alias:        "s",
			ProductAlias: "p",
			Fields:       templateSchemaFields(),
		}).
		ProductOrganization(itdsl.ProductOrganizationOpts{
			Alias: worldOrganizationAlias, ProductAlias: "p",
		}).
		LicenseTemplate(itdsl.LicenseTemplateOpts{
			Alias:        worldTemplateAlias,
			ProductAlias: "p",
			Name:         uniqueTemplateName(),
			Values:       validTemplateValues(),
		}).
		Build()

	return &licenseWorld{
		t:        t,
		state:    state,
		product:  state.Product("p"),
		tenantID: state.Tenant("t").ID,
	}
}

// newLicensedWorld is newLicenseWorld with the template already instantiated
// onto the organization. It is the starting point for reading, adjusting and
// diffing.
func newLicensedWorld(t *testing.T) *licenseWorld {
	t.Helper()
	world := newLicenseWorld(t)
	world.License().Instantiate(world.TemplateID())
	return world
}

func (w *licenseWorld) productID() string { return w.product.ProductID }

func (w *licenseWorld) client() *ct.ClientWithResponses {
	return w.product.OwnerAuthenticatedClient()
}

// OrganizationID is the organization every handle addresses by default.
func (w *licenseWorld) OrganizationID() string {
	return w.state.ProductOrganization(worldOrganizationAlias).ID
}

// TemplateID is the template the world's organization is licensed from.
func (w *licenseWorld) TemplateID() string {
	return w.state.LicenseTemplate(worldTemplateAlias).ID
}

// NewOrganization adds another organization to the same product, for the tests
// whose subject is that one organization's license is its own.
func (w *licenseWorld) NewOrganization() string {
	w.t.Helper()
	return createOrganization(w.t, w.product)
}

// ---------------------------------------------------------------------------
// License handle
// ---------------------------------------------------------------------------

// licenseHandle addresses one organization's license with one credential.
type licenseHandle struct {
	t              *testing.T
	client         *ct.ClientWithResponses
	productID      string
	organizationID string
}

// License addresses the world's own organization, as its owner.
func (w *licenseWorld) License() licenseHandle {
	return licenseHandle{
		t:              w.t,
		client:         w.client(),
		productID:      w.productID(),
		organizationID: w.OrganizationID(),
	}
}

// For addresses another organization, keeping the credential.
func (h licenseHandle) For(organizationID string) licenseHandle {
	h.organizationID = organizationID
	return h
}

// As swaps the credential, for the tests whose subject is a scope.
func (h licenseHandle) As(client *ct.ClientWithResponses) licenseHandle {
	h.client = client
	return h
}

func (h licenseHandle) InstantiateRaw(
	templateID string,
) *ct.InstantiateOrganizationLicenseResponse {
	h.t.Helper()
	resp, err := h.client.InstantiateOrganizationLicenseWithResponse(
		context.Background(),
		h.productID,
		h.organizationID,
		ct.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: templateID},
	)
	require.NoError(h.t, err)
	return resp
}

func (h licenseHandle) Instantiate(templateID string) ct.OrganizationLicenseResponse {
	h.t.Helper()
	resp := h.InstantiateRaw(templateID)
	require.Equal(h.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(h.t, resp.JSON201)
	return *resp.JSON201
}

func (h licenseHandle) GetRaw() *ct.GetOrganizationLicenseResponse {
	h.t.Helper()
	resp, err := h.client.GetOrganizationLicenseWithResponse(
		context.Background(), h.productID, h.organizationID,
	)
	require.NoError(h.t, err)
	return resp
}

func (h licenseHandle) Get() ct.OrganizationLicenseResponse {
	h.t.Helper()
	resp := h.GetRaw()
	require.Equal(h.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(h.t, resp.JSON200)
	return *resp.JSON200
}

func (h licenseHandle) AdjustRaw(
	values ct.LicenseTemplateValues,
) *ct.AdjustOrganizationLicenseResponse {
	h.t.Helper()
	resp, err := h.client.AdjustOrganizationLicenseWithResponse(
		context.Background(),
		h.productID,
		h.organizationID,
		ct.AdjustOrganizationLicenseJSONRequestBody{Values: values},
	)
	require.NoError(h.t, err)
	return resp
}

func (h licenseHandle) Adjust(values ct.LicenseTemplateValues) ct.OrganizationLicenseResponse {
	h.t.Helper()
	resp := h.AdjustRaw(values)
	require.Equal(h.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(h.t, resp.JSON200)
	return *resp.JSON200
}

func (h licenseHandle) DiffRaw() *ct.GetOrganizationLicenseDiffResponse {
	h.t.Helper()
	resp, err := h.client.GetOrganizationLicenseDiffWithResponse(
		context.Background(), h.productID, h.organizationID,
	)
	require.NoError(h.t, err)
	return resp
}

func (h licenseHandle) Diff() ct.OrganizationLicenseDiffResponse {
	h.t.Helper()
	resp := h.DiffRaw()
	require.Equal(h.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(h.t, resp.JSON200)
	return *resp.JSON200
}

// ---------------------------------------------------------------------------
// Usage handle
// ---------------------------------------------------------------------------

type usageHandle struct {
	t              *testing.T
	client         *ct.ClientWithResponses
	productID      string
	organizationID string
}

func (w *licenseWorld) Usage() usageHandle {
	return usageHandle{
		t:              w.t,
		client:         w.client(),
		productID:      w.productID(),
		organizationID: w.OrganizationID(),
	}
}

// For addresses another organization, keeping the credential.
func (h usageHandle) For(organizationID string) usageHandle {
	h.organizationID = organizationID
	return h
}

// As swaps the credential, for the tests whose subject is a scope.
func (h usageHandle) As(client *ct.ClientWithResponses) usageHandle {
	h.client = client
	return h
}

func (h usageHandle) ReportRaw(
	report ct.UsageReportRequest,
) *ct.ReportOrganizationUsageResponse {
	h.t.Helper()
	resp, err := h.client.ReportOrganizationUsageWithResponse(
		context.Background(), h.productID, h.organizationID, report,
	)
	require.NoError(h.t, err)
	return resp
}

func (h usageHandle) Report(report ct.UsageReportRequest) ct.UsageObservationResponse {
	h.t.Helper()
	resp := h.ReportRaw(report)
	require.Equal(h.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(h.t, resp.JSON201)
	return *resp.JSON201
}

// ReportBodyRaw posts a body the generated client cannot express — the typed
// request models value as a float, so a non-numeric value has to be sent raw.
func (h usageHandle) ReportBodyRaw(body string) *ct.ReportOrganizationUsageResponse {
	h.t.Helper()
	resp, err := h.client.ReportOrganizationUsageWithBodyWithResponse(
		context.Background(),
		h.productID,
		h.organizationID,
		"application/json",
		strings.NewReader(body),
	)
	require.NoError(h.t, err)
	return resp
}

func gauge(key string, value float64) ct.UsageReportRequest {
	return ct.UsageReportRequest{Key: key, Value: value}
}

// The one limit in templateSchemaFields. The windowed builders take no key
// because only a limit can carry usage and the world declares exactly one.
const worldLimitKey = "flows"

func windowed(value float64, from, to time.Time) ct.UsageReportRequest {
	return ct.UsageReportRequest{Key: worldLimitKey, Value: value, From: &from, To: &to}
}

func openEnded(value float64, from time.Time) ct.UsageReportRequest {
	return ct.UsageReportRequest{Key: worldLimitKey, Value: value, From: &from}
}

func billingPeriod() (time.Time, time.Time) {
	start := time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}

// ---------------------------------------------------------------------------
// Template and schema handles
// ---------------------------------------------------------------------------

// templateHandle addresses the world's template. It exists so a test can move
// the tier underneath an already-licensed organization, which is the arrange
// half of every isolation and drift case.
type templateHandle struct {
	t          *testing.T
	client     *ct.ClientWithResponses
	productID  string
	templateID string
}

func (w *licenseWorld) Template() templateHandle {
	return templateHandle{
		t: w.t, client: w.client(), productID: w.productID(), templateID: w.TemplateID(),
	}
}

func (h templateHandle) Read() ct.LicenseTemplateResponse {
	h.t.Helper()
	resp, err := h.client.GetLicenseTemplateWithResponse(
		context.Background(), h.productID, h.templateID,
	)
	require.NoError(h.t, err)
	require.Equal(h.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(h.t, resp.JSON200)
	return *resp.JSON200
}

// ReplaceValues rewrites what the tier grants. Every declared license field has
// to be restated, because an omitted one is a removal on a template write.
func (h templateHandle) ReplaceValues(values ct.LicenseTemplateValues) {
	h.t.Helper()
	resp, err := h.client.UpdateLicenseTemplateWithResponse(
		context.Background(),
		h.productID,
		h.templateID,
		ct.UpdateLicenseTemplateJSONRequestBody{Values: &values},
	)
	require.NoError(h.t, err)
	require.Equal(h.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
}

// Archive withdraws the tier. The record is kept, so a license that names it
// keeps resolving.
func (h templateHandle) Archive() {
	h.t.Helper()
	resp, err := h.client.ArchiveLicenseTemplateWithResponse(
		context.Background(), h.productID, h.templateID,
	)
	require.NoError(h.t, err)
	require.Equal(h.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
}

// RedeclareSchema replaces the product's field declaration wholesale.
func (w *licenseWorld) RedeclareSchema(fields []ct.LicenseFieldDeclaration) {
	w.t.Helper()
	resp, err := w.client().UpdateLicenseSchemaWithResponse(
		context.Background(),
		w.productID(),
		ct.UpdateLicenseSchemaJSONRequestBody{Fields: &fields},
	)
	require.NoError(w.t, err)
	require.Equal(w.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
}

// ---------------------------------------------------------------------------
// Assertions and identifiers
// ---------------------------------------------------------------------------

// assertValues compares a whole set of license field values.
//
// Comparing the set rather than one key at a time is what makes "leaves the
// rest alone" a real assertion — a value that appeared or vanished fails here
// and would not fail a per-key check. Numbers are normalised, so a test writes
// 800 rather than the 800.0 a JSON decode produces.
func assertValues(t *testing.T, actual, expected ct.LicenseTemplateValues) {
	t.Helper()
	assert.Equal(t, normaliseValues(expected), normaliseValues(actual))
}

func normaliseValues(values ct.LicenseTemplateValues) map[string]any {
	out := make(map[string]any, len(values))
	for name, value := range values {
		switch typed := value.(type) {
		case int:
			out[name] = float64(typed)
		case int32:
			out[name] = float64(typed)
		case int64:
			out[name] = float64(typed)
		case float32:
			out[name] = float64(typed)
		default:
			out[name] = value
		}
	}
	return out
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

// missingOrganizationID is a well-formed identifier for an organization that
// was never created. It has to be a real KSUID: request validation rejects a
// malformed path parameter with a 400 before any handler runs, which would
// prove the contract's pattern rather than the handler's answer.
func missingOrganizationID() string {
	return ids.MustNew("org")
}

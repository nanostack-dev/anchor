package license_ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// Creating an organization can carry the license it starts on, so a product
// onboarding a customer writes both as one unit. The subject of these tests is
// that unit: what the create call answers, and what is left behind when the
// license is refused.

// createHandle creates organizations through a Product API key holding only
// organization:create — the scope the route demands, and the one the embedded
// license rides on.
type createHandle struct {
	t         *testing.T
	client    *ct.ClientWithResponses
	productID string
}

func (w *licenseWorld) Create() createHandle {
	w.t.Helper()
	client, _ := w.product.CreateAPIKeyClientWithScopes(
		[]string{"organization:create", "organization:read"},
	)
	return createHandle{t: w.t, client: client, productID: w.productID()}
}

func (h createHandle) WithLicenseRaw(
	name string, body ct.CreateProductOrganizationJSONRequestBody,
) *ct.CreateProductOrganizationResponse {
	h.t.Helper()
	body.Name = name
	resp, err := h.client.CreateProductOrganizationWithResponse(
		context.Background(), h.productID, body,
	)
	require.NoError(h.t, err)
	return resp
}

func (h createHandle) WithLicense(
	name string, body ct.CreateProductOrganizationJSONRequestBody,
) ct.ProductOrganizationResponse {
	h.t.Helper()
	resp := h.WithLicenseRaw(name, body)
	require.Equal(h.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	require.NotNil(h.t, resp.JSON201)
	return *resp.JSON201
}

// CountNamed reports how many organizations of the product carry that name. It
// is how a rolled-back create is told apart from a create that never ran.
func (h createHandle) CountNamed(name string) int {
	h.t.Helper()
	resp, err := h.client.SearchProductOrganizationsWithResponse(
		context.Background(),
		h.productID,
		ct.SearchProductOrganizationsJSONRequestBody{
			Filter: &ct.OrganizationFilter{Names: []string{name}},
		},
	)
	require.NoError(h.t, err)
	require.Equal(h.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.NotNil(h.t, resp.JSON200)
	return len(resp.JSON200.Items)
}

func uniqueOrganizationName() string {
	return "org_" + ids.MustNew("ct")
}

func licenseBody(templateID string) ct.CreateProductOrganizationJSONRequestBody {
	return ct.CreateProductOrganizationJSONRequestBody{
		License: &ct.OrganizationLicenseInstantiateRequest{TemplateId: templateID},
	}
}

func TestCreateOrganizationWithLicense(t *testing.T) {
	t.Run("stamps the template onto the organization it creates", func(t *testing.T) {
		w := newLicenseWorld(t)

		before := time.Now().Add(-time.Second)
		created := w.Create().WithLicense(uniqueOrganizationName(), licenseBody(w.TemplateID()))

		require.NotNil(t, created.License, "the create answered no license")
		assert.Equal(t, created.Id, created.License.OrganizationId)
		assert.Equal(t, w.TemplateID(), created.License.TemplateId)
		assertValues(t, created.License.Values, validTemplateValues())
		assert.WithinRange(t, created.License.InstantiatedAt, before, time.Now().Add(time.Second))
	})

	t.Run("the license it answers is the one the license route reads", func(t *testing.T) {
		w := newLicenseWorld(t)

		created := w.Create().WithLicense(uniqueOrganizationName(), licenseBody(w.TemplateID()))
		read := w.License().For(created.Id).Get()

		require.NotNil(t, created.License)
		assert.Equal(t, created.License.Id, read.Id)
		assertValues(t, read.Values, created.License.Values)
	})

	t.Run("the license is recorded in the organization's history", func(t *testing.T) {
		w := newLicenseWorld(t)

		created := w.Create().WithLicense(uniqueOrganizationName(), licenseBody(w.TemplateID()))
		history := w.License().For(created.Id).History()

		assert.NotEmpty(t, history.Items, "an instantiation writes its own history")
	})

	t.Run("asking for no license leaves the organization unlicensed", func(t *testing.T) {
		w := newLicenseWorld(t)

		created := w.Create().WithLicense(
			uniqueOrganizationName(), ct.CreateProductOrganizationJSONRequestBody{},
		)

		assert.Nil(t, created.License)
		assert.Equal(t, http.StatusNotFound, w.License().For(created.Id).GetRaw().StatusCode())
	})

	t.Run("a template that does not exist leaves no organization behind", func(t *testing.T) {
		w := newLicenseWorld(t)
		name := uniqueOrganizationName()

		body := licenseBody(missingTemplateID())
		resp := w.Create().WithLicenseRaw(name, body)

		require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
		assert.Zero(t, w.Create().CountNamed(name), "the organization was not rolled back")
	})

	t.Run("an archived template leaves no organization behind", func(t *testing.T) {
		w := newLicenseWorld(t)
		w.Template().Archive()
		name := uniqueOrganizationName()

		resp := w.Create().WithLicenseRaw(name, licenseBody(w.TemplateID()))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertAPIError(t, resp.JSON400.Errors, "LICENSE_TEMPLATE_ARCHIVED")
		assert.Zero(t, w.Create().CountNamed(name), "the organization was not rolled back")
	})

	t.Run("another product's template leaves no organization behind", func(t *testing.T) {
		first := newLicenseWorld(t)
		second := newLicenseWorld(t)
		name := uniqueOrganizationName()

		resp := first.Create().WithLicenseRaw(name, licenseBody(second.TemplateID()))

		require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
		assert.Zero(t, first.Create().CountNamed(name), "the organization was not rolled back")
	})

	t.Run("licenses the organization it creates with a founding member", func(t *testing.T) {
		w := newLicenseWorld(t)
		founder := w.FoundingMember()
		name := uniqueOrganizationName()

		body := licenseBody(w.TemplateID())
		body.FoundingMember = &ct.FoundingMemberRequest{
			ProductUserId: founder.ProductUserID, RoleId: founder.RoleID,
		}
		created := w.Create().WithLicense(name, body)

		require.NotNil(t, created.License, "the create answered no license")
		assert.Equal(t, created.Id, created.License.OrganizationId)
		assertValues(t, created.License.Values, validTemplateValues())
	})

	t.Run("a refused template leaves no founding membership behind", func(t *testing.T) {
		w := newLicenseWorld(t)
		founder := w.FoundingMember()
		name := uniqueOrganizationName()

		body := licenseBody(missingTemplateID())
		body.FoundingMember = &ct.FoundingMemberRequest{
			ProductUserId: founder.ProductUserID, RoleId: founder.RoleID,
		}
		resp := w.Create().WithLicenseRaw(name, body)
		require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))

		assert.Zero(t, w.Create().CountNamed(name), "the organization was not rolled back")

		// The membership went with it: the same user can still found an
		// organization, which the idempotent path would refuse to create had
		// the rolled-back one survived.
		retry := uniqueOrganizationName()
		retryBody := ct.CreateProductOrganizationJSONRequestBody{
			FoundingMember: &ct.FoundingMemberRequest{
				ProductUserId: founder.ProductUserID, RoleId: founder.RoleID,
			},
		}
		second := w.Create().WithLicense(retry, retryBody)
		assert.Equal(t, retry, second.Name)
	})
}

// foundingMember is the product user and role a founding-member create names.
type foundingMember struct {
	ProductUserID string
	RoleID        string
}

// FoundingMember adds a product user and a role to the world, for the create
// calls whose subject is the founding-member path.
func (w *licenseWorld) FoundingMember() foundingMember {
	w.t.Helper()
	state := itdsl.Given(w.t).
		ExistingProduct(itdsl.ExistingProductOpts{Alias: "p", Context: w.product}).
		ProductUser(itdsl.ProductUserOpts{Alias: "u", ProductAlias: "p"}).
		ProductRole(itdsl.ProductRoleOpts{Alias: "r", ProductAlias: "p"}).
		Build()
	return foundingMember{
		ProductUserID: state.ProductUser("u").ID,
		RoleID:        state.ProductRole("r").ID,
	}
}

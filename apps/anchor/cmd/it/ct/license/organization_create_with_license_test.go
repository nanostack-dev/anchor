package license_ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// organizationBody names a new organization, optionally asking for the license
// it starts on.
func organizationBody(
	name string, templateID *string,
) ct.CreateProductOrganizationJSONRequestBody {
	body := ct.CreateProductOrganizationJSONRequestBody{Name: name}
	if templateID != nil {
		body.License = &ct.OrganizationLicenseInstantiateRequest{TemplateId: *templateID}
	}
	return body
}

// Organizations drives the world's product with a key holding only the scopes
// an organization read and write need. It deliberately excludes
// organization_license:create — the embedded license rides on
// organization:create.
func (w *licenseWorld) Organizations() itdsl.OrganizationClient {
	return w.product.Organizations("organization:create", "organization:read")
}

func TestOrganizationReadIncludesLicense(t *testing.T) {
	t.Run("get without include leaves the license out", func(t *testing.T) {
		w := newLicenseWorld(t)
		created := w.Organizations().Create(organizationBody(itdsl.UniqueOrganizationName(), new(w.TemplateID())))

		read := w.Organizations().Get(created.Id)

		// Absent says nothing about whether the organization holds one — this
		// organization does.
		assert.Nil(t, read.License)
	})

	t.Run("get with include=license carries it", func(t *testing.T) {
		w := newLicenseWorld(t)
		created := w.Organizations().Create(organizationBody(itdsl.UniqueOrganizationName(), new(w.TemplateID())))

		read := w.Organizations().Get(created.Id, ct.OrganizationIncludeLicense)

		require.NotNil(t, read.License)
		assert.Equal(t, created.License.Id, read.License.Id)
		assert.Equal(t, w.TemplateID(), read.License.TemplateId)
		assertValues(t, read.License.Values, validTemplateValues())
	})

	t.Run("the include carries no usage; the license route computes that", func(t *testing.T) {
		w := newLicenseWorld(t)
		created := w.Organizations().Create(organizationBody(itdsl.UniqueOrganizationName(), new(w.TemplateID())))

		read := w.Organizations().Get(created.Id, ct.OrganizationIncludeLicense)

		require.NotNil(t, read.License)
		assert.Nil(t, read.License.Usage)
		assert.NotNil(t, w.License().For(created.Id).Get().Usage)
	})

	t.Run("an unlicensed organization reads back without a license", func(t *testing.T) {
		w := newLicenseWorld(t)
		created := w.Organizations().Create(
			organizationBody(itdsl.UniqueOrganizationName(), nil),
		)

		read := w.Organizations().Get(created.Id, ct.OrganizationIncludeLicense)

		assert.Nil(t, read.License)
	})

	t.Run("search with include=license carries one per organization", func(t *testing.T) {
		w := newLicenseWorld(t)
		name := itdsl.UniqueOrganizationName()
		first := w.Organizations().Create(organizationBody(name, new(w.TemplateID())))
		second := w.Organizations().Create(organizationBody(name, nil))

		items := w.Organizations().SearchNamed(name, ct.OrganizationIncludeLicense)

		require.Len(t, items, 2)
		byID := map[string]ct.ProductOrganizationResponse{}
		for _, item := range items {
			byID[item.Id] = item
		}
		// One page, two organizations, and only the licensed one carries a
		// license — the unlicensed one is not given the other's.
		require.NotNil(t, byID[first.Id].License)
		assert.Equal(t, first.Id, byID[first.Id].License.OrganizationId)
		assert.Nil(t, byID[second.Id].License)
	})

	t.Run("search without include leaves every license out", func(t *testing.T) {
		w := newLicenseWorld(t)
		name := itdsl.UniqueOrganizationName()
		w.Organizations().Create(organizationBody(name, new(w.TemplateID())))

		items := w.Organizations().SearchNamed(name)

		require.Len(t, items, 1)
		assert.Nil(t, items[0].License)
	})

	t.Run("another product's organization is not reachable through the include", func(t *testing.T) {
		first := newLicenseWorld(t)
		second := newLicenseWorld(t)
		licensed := second.Organizations().Create(
			organizationBody(itdsl.UniqueOrganizationName(), new(second.TemplateID())),
		)

		resp := first.Organizations().GetRaw(licensed.Id, ct.OrganizationIncludeLicense)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
}

func TestCreateOrganizationWithLicense(t *testing.T) {
	t.Run("stamps the template onto the organization it creates", func(t *testing.T) {
		w := newLicenseWorld(t)

		before := time.Now().Add(-time.Second)
		created := w.Organizations().Create(organizationBody(itdsl.UniqueOrganizationName(), new(w.TemplateID())))

		require.NotNil(t, created.License, "the create answered no license")
		assert.Equal(t, created.Id, created.License.OrganizationId)
		assert.Equal(t, w.TemplateID(), created.License.TemplateId)
		assertValues(t, created.License.Values, validTemplateValues())
		assert.WithinRange(t, created.License.InstantiatedAt, before, time.Now().Add(time.Second))
	})

	t.Run("the license it answers is the one the license route reads", func(t *testing.T) {
		w := newLicenseWorld(t)

		created := w.Organizations().Create(organizationBody(itdsl.UniqueOrganizationName(), new(w.TemplateID())))
		read := w.License().For(created.Id).Get()

		require.NotNil(t, created.License)
		assert.Equal(t, created.License.Id, read.Id)
		assertValues(t, read.Values, created.License.Values)
	})

	t.Run("the license is recorded in the organization's history", func(t *testing.T) {
		w := newLicenseWorld(t)

		created := w.Organizations().Create(organizationBody(itdsl.UniqueOrganizationName(), new(w.TemplateID())))
		history := w.License().For(created.Id).History()

		assert.NotEmpty(t, history.Items, "an instantiation writes its own history")
	})

	t.Run("asking for no license leaves the organization unlicensed", func(t *testing.T) {
		w := newLicenseWorld(t)

		created := w.Organizations().Create(
			organizationBody(itdsl.UniqueOrganizationName(), nil),
		)

		assert.Nil(t, created.License)
		assert.Equal(t, http.StatusNotFound, w.License().For(created.Id).GetRaw().StatusCode())
	})

	t.Run("a template that does not exist is a bad request", func(t *testing.T) {
		w := newLicenseWorld(t)
		name := itdsl.UniqueOrganizationName()

		body := organizationBody(name, new(missingTemplateID()))
		resp := w.Organizations().CreateRaw(body)

		// Not a 404: the call addressed the organization collection, which exists.
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertAPIError(t, resp.JSON400.Errors, "ORGANIZATION_LICENSE_TEMPLATE_NOT_FOUND")
		assert.Zero(t, w.Organizations().CountNamed(name), "no organization was left behind")
	})

	t.Run("an archived template leaves no organization behind", func(t *testing.T) {
		w := newLicenseWorld(t)
		w.Template().Archive()
		name := itdsl.UniqueOrganizationName()

		resp := w.Organizations().CreateRaw(organizationBody(name, new(w.TemplateID())))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertAPIError(t, resp.JSON400.Errors, "LICENSE_TEMPLATE_ARCHIVED")
		assert.Zero(t, w.Organizations().CountNamed(name), "the organization was not rolled back")
	})

	t.Run("another product's template is refused the same way", func(t *testing.T) {
		first := newLicenseWorld(t)
		second := newLicenseWorld(t)
		name := itdsl.UniqueOrganizationName()

		// The same answer as a template that does not exist: saying which would
		// leak that the identifier is real.
		resp := first.Organizations().CreateRaw(organizationBody(name, new(second.TemplateID())))

		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON400)
		assertAPIError(t, resp.JSON400.Errors, "ORGANIZATION_LICENSE_TEMPLATE_NOT_FOUND")
		assert.Zero(t, first.Organizations().CountNamed(name), "no organization was left behind")
	})

	t.Run("licenses the organization it creates with a founding member", func(t *testing.T) {
		w := newLicenseWorld(t)
		founder := w.FoundingMember()
		name := itdsl.UniqueOrganizationName()

		body := organizationBody(name, new(w.TemplateID()))
		body.FoundingMember = &ct.FoundingMemberRequest{
			ProductUserId: founder.ProductUserID, RoleId: founder.RoleID,
		}
		created := w.Organizations().Create(body)

		require.NotNil(t, created.License, "the create answered no license")
		assert.Equal(t, created.Id, created.License.OrganizationId)
		assertValues(t, created.License.Values, validTemplateValues())
	})

	t.Run("a refused template writes no founding membership", func(t *testing.T) {
		w := newLicenseWorld(t)
		founder := w.FoundingMember()
		name := itdsl.UniqueOrganizationName()

		body := organizationBody(name, new(missingTemplateID()))
		body.FoundingMember = &ct.FoundingMemberRequest{
			ProductUserId: founder.ProductUserID, RoleId: founder.RoleID,
		}
		resp := w.Organizations().CreateRaw(body)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))

		assert.Zero(t, w.Organizations().CountNamed(name), "no organization was left behind")

		// The same user can still found one, which the idempotent path would
		// refuse had any membership survived.
		retry := itdsl.UniqueOrganizationName()
		retryBody := organizationBody(retry, nil)
		retryBody.FoundingMember = &ct.FoundingMemberRequest{
			ProductUserId: founder.ProductUserID, RoleId: founder.RoleID,
		}
		second := w.Organizations().Create(retryBody)
		assert.Equal(t, retry, second.Name)
	})
}

type foundingMember struct {
	ProductUserID string
	RoleID        string
}

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

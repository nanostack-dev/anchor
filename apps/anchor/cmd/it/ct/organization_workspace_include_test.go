package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

func TestOrganizationReadIncludesWorkspaces(t *testing.T) {
	t.Parallel()
	product := createTestProductContext(t)
	org := product.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{Name: ids.MustNew("org")})
	plain := product.Organizations().GetRaw(org.Id)
	require.NotContains(t, string(plain.Body), `"workspaces"`)
	empty := product.Organizations().GetRaw(org.Id, ct.OrganizationIncludeWorkspace)
	require.Equal(t, http.StatusOK, empty.StatusCode())
	require.Contains(t, string(empty.Body), `"workspaces":[]`)
	require.NotNil(t, empty.JSON200.Workspaces)
	require.Empty(t, *empty.JSON200.Workspaces)
	created := product.Workspaces(org.Id).Create(ct.CreateOrganizationWorkspaceJSONRequestBody{
		Name: "Default", IsDefault: new(true),
	})
	other := product.Workspaces(org.Id).Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Production"})
	read := product.Organizations().Get(org.Id, ct.OrganizationIncludeWorkspace)
	require.NotNil(t, read.Workspaces)
	require.Len(t, *read.Workspaces, 2)
	byID := map[string]ct.ProductWorkspaceResponse{}
	for _, item := range *read.Workspaces {
		byID[item.Id] = item
	}
	require.Equal(t, created.Id, byID[created.Id].Id)
	require.True(t, *byID[created.Id].IsDefault)
	require.Equal(t, other.Id, byID[other.Id].Id)
	require.False(t, *byID[other.Id].IsDefault)
}

func TestOrganizationReadIncludesWorkspaceAndLicense(t *testing.T) {
	t.Parallel()
	w := newLicenseWorld(t)
	name := itdsl.UniqueOrganizationName()
	body := organizationBody(name, new(w.TemplateID()))
	body.CreateDefaultWorkspace = new(true)
	licensed := w.Organizations().Create(body)
	empty := w.Organizations().Create(organizationBody(name, nil))
	read := w.Organizations().Get(licensed.Id, ct.OrganizationIncludeWorkspace, ct.OrganizationIncludeLicense)
	require.NotNil(t, read.License)
	require.Equal(t, licensed.License.Id, read.License.Id)
	require.Nil(t, read.License.Usage)
	require.NotNil(t, read.Workspaces)
	require.Len(t, *read.Workspaces, 1)
	page := w.Organizations().SearchNamed(name, ct.OrganizationIncludeWorkspace, ct.OrganizationIncludeLicense)
	require.Len(t, page, 2)
	for _, item := range page {
		require.NotNil(t, item.Workspaces)
		if item.Id == empty.Id {
			require.Empty(t, *item.Workspaces)
			require.Nil(t, item.License)
		} else {
			require.Equal(t, licensed.Id, item.Id)
			require.Len(t, *item.Workspaces, 1)
			require.Equal(t, licensed.License.Id, item.License.Id)
		}
	}
}

func TestOrganizationWorkspaceIncludeIsProductScoped(t *testing.T) {
	t.Parallel()
	owner := createTestProductContext(t)
	foreign := createTestProductContext(t)
	name := ids.MustNew("org")
	org := owner.Organizations().
		Create(ct.CreateProductOrganizationJSONRequestBody{Name: name, CreateDefaultWorkspace: new(true)})
	foreignOrg := foreign.Organizations().
		Create(ct.CreateProductOrganizationJSONRequestBody{Name: name, CreateDefaultWorkspace: new(true)})
	refused := owner.Organizations().
		GetRaw(foreignOrg.Id, ct.OrganizationIncludeWorkspace, ct.OrganizationIncludeLicense)
	require.Equal(t, http.StatusNotFound, refused.StatusCode())
	page := owner.Organizations().SearchNamed(name, ct.OrganizationIncludeWorkspace)
	require.Len(t, page, 1)
	require.Equal(t, org.Id, page[0].Id)
	for _, item := range *page[0].Workspaces {
		require.Equal(t, org.Id, item.OrganizationId)
	}
}

func TestDefaultWorkspaceUsesExistingCreateAPI(t *testing.T) {
	t.Parallel()
	product := createTestProductContext(t)
	org := product.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{Name: ids.MustNew("org")})
	legacy := product.Workspaces(org.Id).Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Default"})
	body := ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Default", IsDefault: new(true)}
	first := product.Workspaces(org.Id).Create(body)
	require.Equal(t, legacy.Id, first.Id)
	require.True(t, *first.IsDefault)
	product.Workspaces(org.Id).Update(first.Id, ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: "Renamed default"})
	again := product.Workspaces(org.Id).Create(body)
	require.Equal(t, first.Id, again.Id)
	require.Equal(t, "Renamed default", again.Name)
	require.True(t, *again.IsDefault)
	read := product.Organizations().Get(org.Id, ct.OrganizationIncludeWorkspace)
	require.Len(t, *read.Workspaces, 1)
}

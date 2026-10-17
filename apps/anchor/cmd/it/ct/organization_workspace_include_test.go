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

	t.Run("Organization read includes workspaces", func(t *testing.T) {
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
			Name: "Development",
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
		require.Equal(t, other.Id, byID[other.Id].Id)
	})
}

func TestOrganizationReadIncludesWorkspaceAndLicense(t *testing.T) {
	t.Parallel()

	t.Run("Organization read includes workspace and license", func(t *testing.T) {
		t.Parallel()

		w := newLicenseWorld(t)
		name := itdsl.UniqueOrganizationName()
		body := organizationBody(name, new(w.TemplateID()))
		body.Workspace = &ct.ProductWorkspaceRequest{Name: "Production"}
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
	})
}

func TestOrganizationWorkspaceIncludeIsProductScoped(t *testing.T) {
	t.Parallel()

	t.Run("Organization workspace include is product scoped", func(t *testing.T) {
		t.Parallel()

		owner := createTestProductContext(t)
		foreign := createTestProductContext(t)
		name := ids.MustNew("org")
		org := owner.Organizations().
			Create(ct.CreateProductOrganizationJSONRequestBody{Name: name, Workspace: &ct.ProductWorkspaceRequest{Name: "Production"}})
		foreignOrg := foreign.Organizations().
			Create(ct.CreateProductOrganizationJSONRequestBody{Name: name, Workspace: &ct.ProductWorkspaceRequest{Name: "Production"}})
		refused := owner.Organizations().
			GetRaw(foreignOrg.Id, ct.OrganizationIncludeWorkspace, ct.OrganizationIncludeLicense)
		require.Equal(t, http.StatusNotFound, refused.StatusCode())
		page := owner.Organizations().SearchNamed(name, ct.OrganizationIncludeWorkspace)
		require.Len(t, page, 1)
		require.Equal(t, org.Id, page[0].Id)
		for _, item := range *page[0].Workspaces {
			require.Equal(t, org.Id, item.OrganizationId)
		}
	})
}

func TestOrganizationWorkspacesHaveNoSpecialIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Organization workspaces have no special identity", func(t *testing.T) {
		t.Parallel()

		product := createTestProductContext(t)
		org := product.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{Name: ids.MustNew("org")})
		first := product.Workspaces(org.Id).Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Default"})
		second := product.Workspaces(org.Id).Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Production"})
		product.Workspaces(org.Id).Update(first.Id, ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: "Development"})
		read := product.Organizations().GetRaw(org.Id, ct.OrganizationIncludeWorkspace)
		require.NotContains(t, string(read.Body), `"is_default"`)
		require.Len(t, *read.JSON200.Workspaces, 2)
		product.Workspaces(org.Id).Delete(first.Id)
		product.Workspaces(org.Id).Delete(second.Id)
		empty := product.Organizations().GetRaw(org.Id, ct.OrganizationIncludeWorkspace)
		require.Contains(t, string(empty.Body), `"workspaces":[]`)
	})
}

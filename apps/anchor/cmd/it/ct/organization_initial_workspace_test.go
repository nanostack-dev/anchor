package ct_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/internal/domain/organization"
)

func TestOrganizationCreateWithInitialWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Creating an organization with initial workspace succeeds", func(t *testing.T) {
		t.Parallel()

		product := createTestProductContext(t)
		sink := product.CaptureEvents()
		created := product.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{
			Name: ids.MustNew(
				"org",
			),
			Workspace: &ct.ProductWorkspaceRequest{Name: "Engineering", Description: new("API tests")},
		})
		workspaces := product.Workspaces(created.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
		require.Len(t, workspaces.Items, 1)
		initial := workspaces.Items[0]
		require.Equal(t, "Engineering", initial.Name)
		require.Equal(t, new("API tests"), initial.Description)
		sink.WaitFor("organization.created", map[string]string{"organization_id": created.Id})
		sink.WaitFor("workspace.created", map[string]string{"organization_id": created.Id, "workspace_id": initial.Id})
	})
}

func TestInitialWorkspaceSharesOrganizationTransaction(t *testing.T) {
	t.Parallel()

	t.Run("Initial workspace shares organization transaction", func(t *testing.T) {
		t.Parallel()

		product := createTestProductContext(t)
		aborted := errors.New("abort caller transaction")
		var created organization.Organization
		err := initialWorkspaceTx.InTx(context.Background(), func(ctx context.Context) error {
			var createErr error
			created, createErr = initialWorkspaceOrganizations.Create(ctx, organization.CreateOrganizationInput{
				ProductID: product.ProductID,
				Name:      ids.MustNew("org"),
				Workspace: &organization.InitialWorkspaceInput{Name: "Engineering"},
			})
			if createErr != nil {
				return createErr
			}
			return aborted
		})
		require.ErrorIs(t, err, aborted)
		require.NotEmpty(t, created.ID)
		require.Equal(t, http.StatusNotFound, product.Organizations().GetRaw(created.ID).StatusCode())
		workspaces := product.Workspaces(created.ID).SearchRaw(ct.SearchOrganizationWorkspacesJSONRequestBody{})
		require.Equal(t, http.StatusNotFound, workspaces.StatusCode())
	})
}

func TestOrganizationCreateWithoutWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Creating an organization without workspace succeeds", func(t *testing.T) {
		t.Parallel()

		product := createTestProductContext(t)
		created := product.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{Name: ids.MustNew("org")})
		workspaces := product.Workspaces(created.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
		require.Empty(t, workspaces.Items)
	})
}

func TestOrganizationFoundingMemberRetryDoesNotRecreateWorkspaces(t *testing.T) {
	t.Parallel()

	t.Run("Organization founding member retry does not recreate workspaces", func(t *testing.T) {
		t.Parallel()

		product := createTestProductContext(t)
		state := itdsl.Given(t).
			ExistingProduct(itdsl.ExistingProductOpts{Alias: "p", Context: product}).
			ProductUser(itdsl.ProductUserOpts{Alias: "u", ProductAlias: "p"}).
			ProductRole(itdsl.ProductRoleOpts{Alias: "r", ProductAlias: "p"}).Build()
		body := ct.CreateProductOrganizationJSONRequestBody{
			Name:      ids.MustNew("org"),
			Workspace: &ct.ProductWorkspaceRequest{Name: "Engineering"},
			FoundingMember: &ct.FoundingMemberRequest{
				ProductUserId: state.ProductUser("u").ID,
				RoleId:        new(state.ProductRole("r").ID),
			},
		}
		created := product.Organizations().Create(body)
		original := product.Workspaces(created.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
		require.Len(t, original.Items, 1)
		product.Workspaces(created.Id).
			Update(original.Items[0].Id, ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: "Renamed"})
		retried := product.Organizations().Create(body)
		require.Equal(t, created.Id, retried.Id)
		unchanged := product.Workspaces(created.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
		require.Len(t, unchanged.Items, 1)
		require.Equal(t, original.Items[0].Id, unchanged.Items[0].Id)
		require.Equal(t, "Renamed", unchanged.Items[0].Name)
		product.Workspaces(created.Id).Delete(original.Items[0].Id)
		again := product.Organizations().Create(body)
		require.Equal(t, created.Id, again.Id)
		empty := product.Organizations().Get(created.Id, ct.OrganizationIncludeWorkspace)
		require.Empty(t, *empty.Workspaces)
	})
}

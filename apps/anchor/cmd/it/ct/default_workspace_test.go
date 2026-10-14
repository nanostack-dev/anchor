package ct_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/internal/domain/organization"
	"anchor/internal/domain/workspace"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/require"
)

func TestOrganizationCreateWithDefaultWorkspace(t *testing.T) {
	t.Parallel()
	product := createTestProductContext(t)
	sink := product.CaptureEvents()
	created := product.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{
		Name: "Default workspace " + ids.MustNew("org"), CreateDefaultWorkspace: new(true),
	})
	workspaces := product.Workspaces(created.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
	require.Len(t, workspaces.Items, 1)
	defaultWorkspace := workspaces.Items[0]
	require.Equal(t, "Default", defaultWorkspace.Name)
	require.NotNil(t, defaultWorkspace.IsDefault)
	require.True(t, *defaultWorkspace.IsDefault)
	sink.WaitFor("organization.created", map[string]string{"organization_id": created.Id})
	sink.WaitFor(
		"workspace.created",
		map[string]string{"organization_id": created.Id, "workspace_id": defaultWorkspace.Id},
	)
}

func TestDefaultWorkspaceSharesOrganizationTransaction(t *testing.T) {
	t.Parallel()
	product := createTestProductContext(t)
	aborted := errors.New("abort caller transaction")
	var created organization.Organization
	err := defaultWorkspaceTx.InTx(context.Background(), func(ctx context.Context) error {
		var createErr error
		created, createErr = defaultWorkspaceOrganizations.Create(ctx, organization.CreateOrganizationInput{
			ProductID: product.ProductID, Name: ids.MustNew("org"), CreateDefaultWorkspace: true,
		})
		if createErr != nil {
			return createErr
		}
		return aborted
	})
	require.ErrorIs(t, err, aborted)
	require.NotEmpty(t, created.ID)
	missing := product.Organizations().GetRaw(created.ID)
	require.Equal(t, http.StatusNotFound, missing.StatusCode())
}

func TestOrganizationCreateDefaultWorkspaceIsOptional(t *testing.T) {
	t.Parallel()
	product := createTestProductContext(t)
	for _, choice := range []struct {
		name string
		flag *bool
	}{{name: "omitted"}, {name: "false", flag: new(false)}} {
		t.Run(choice.name, func(t *testing.T) {
			created := product.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{
				Name: ids.MustNew("org"), CreateDefaultWorkspace: choice.flag,
			})
			workspaces := product.Workspaces(created.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
			require.Empty(t, workspaces.Items)
		})
	}
}

func TestOrganizationFoundingMemberRetryEnsuresDefaultWorkspace(t *testing.T) {
	t.Parallel()
	product := createTestProductContext(t)
	state := itdsl.Given(t).
		ExistingProduct(itdsl.ExistingProductOpts{Alias: "p", Context: product}).
		ProductUser(itdsl.ProductUserOpts{Alias: "u", ProductAlias: "p"}).
		ProductRole(itdsl.ProductRoleOpts{Alias: "r", ProductAlias: "p"}).Build()
	body := ct.CreateProductOrganizationJSONRequestBody{
		Name: ids.MustNew("org"),
		FoundingMember: &ct.FoundingMemberRequest{
			ProductUserId: state.ProductUser("u").ID,
			RoleId:        state.ProductRole("r").ID,
		},
	}
	created := product.Organizations().Create(body)
	body.CreateDefaultWorkspace = new(true)
	retried := product.Organizations().Create(body)
	require.Equal(t, created.Id, retried.Id)
	workspaces := product.Workspaces(created.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
	require.Len(t, workspaces.Items, 1)
	require.NotNil(t, workspaces.Items[0].IsDefault)
	require.True(t, *workspaces.Items[0].IsDefault)
	again := product.Organizations().Create(body)
	require.Equal(t, created.Id, again.Id)
	unchanged := product.Workspaces(created.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
	require.Len(t, unchanged.Items, 1)
	require.Equal(t, workspaces.Items[0].Id, unchanged.Items[0].Id)
}

// A workspace edit can have read its entity before default provisioning designates it.
// Persist that stale edit after provisioning to pin the actual lost-update interleaving.
func TestDefaultWorkspaceDesignationSurvivesStaleEdit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	product := createTestProductContext(t)
	org := product.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{Name: ids.MustNew("org")})
	original := product.Workspaces(org.Id).Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Default"})
	before, err := defaultWorkspaceRepository.FindByID(ctx, product.ProductID, org.Id, original.Id)
	require.NoError(t, err)
	require.True(t, before.IsPresent())
	stale := before.Value()
	require.False(t, stale.IsDefault)
	_, err = defaultWorkspaceService.Create(
		ctx,
		workspace.CreateWorkspaceInput{
			Name:           "Default",
			IsDefault:      true,
			ProductID:      product.ProductID,
			OrganizationID: org.Id,
		},
	)
	require.NoError(t, err)
	stale.Name = "Renamed " + ids.MustNew("ws")
	stale.Description = new("Concurrent edit")
	updated, err := defaultWorkspaceRepository.Update(ctx, product.ProductID, org.Id, stale)
	require.NoError(t, err)
	require.True(t, updated.IsDefault)
	require.Equal(t, stale.Name, updated.Name)
	require.Equal(t, stale.Description, updated.Description)
	ensured, err := defaultWorkspaceService.Create(
		ctx,
		workspace.CreateWorkspaceInput{
			Name:           "Default",
			IsDefault:      true,
			ProductID:      product.ProductID,
			OrganizationID: org.Id,
		},
	)
	require.NoError(t, err)
	require.Equal(t, original.Id, ensured.ID)
	require.Equal(t, stale.Name, ensured.Name)
	workspaces := product.Workspaces(org.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
	require.Len(t, workspaces.Items, 1)
}

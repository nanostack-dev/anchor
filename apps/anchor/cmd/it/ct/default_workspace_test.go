package ct_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/internal/domain/organization"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
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

func TestDefaultWorkspaceStartupSweep(t *testing.T) {
	// Not parallel: startup reconciliation visits organizations across every product.
	ctx := context.Background()
	first := createTestProductContext(t)
	second := createTestProductContext(t)
	legacy := first.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{Name: ids.MustNew("org")})
	legacyWorkspace := first.Workspaces(legacy.Id).
		Create(ct.CreateOrganizationWorkspaceJSONRequestBody{Name: "Default"})
	sink := first.CaptureEvents()
	organizations := make([]ct.ProductOrganizationResponse, 0, 205)
	for range 205 {
		organizations = append(
			organizations,
			second.Organizations().Create(ct.CreateProductOrganizationJSONRequestBody{Name: ids.MustNew("org")}),
		)
	}
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return defaultWorkspaceStartup.Run(groupCtx) })
	group.Go(func() error { return defaultWorkspaceStartup.Run(groupCtx) })
	require.NoError(t, group.Wait())
	legacyAfter := first.Workspaces(legacy.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
	require.Len(t, legacyAfter.Items, 1)
	require.Equal(t, legacyWorkspace.Id, legacyAfter.Items[0].Id)
	require.NotNil(t, legacyAfter.Items[0].IsDefault)
	require.True(t, *legacyAfter.Items[0].IsDefault)
	sink.WaitFor(
		"workspace.updated",
		map[string]string{"organization_id": legacy.Id, "workspace_id": legacyWorkspace.Id},
	)
	for _, org := range organizations {
		workspaces := second.Workspaces(org.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
		require.Len(t, workspaces.Items, 1)
		require.NotNil(t, workspaces.Items[0].IsDefault)
		require.True(t, *workspaces.Items[0].IsDefault)
	}
	first.Workspaces(legacy.Id).
		Update(legacyWorkspace.Id, ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: "Renamed " + ids.MustNew("ws")})
	require.NoError(t, defaultWorkspaceStartup.Run(ctx))
	afterRestart := first.Workspaces(legacy.Id).Search(ct.SearchOrganizationWorkspacesJSONRequestBody{})
	require.Len(t, afterRestart.Items, 1)
	require.Equal(t, legacyWorkspace.Id, afterRestart.Items[0].Id)
	require.NotNil(t, afterRestart.Items[0].IsDefault)
	require.True(t, *afterRestart.Items[0].IsDefault)
}

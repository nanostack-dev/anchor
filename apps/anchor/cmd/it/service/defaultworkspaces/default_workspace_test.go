package defaultworkspaces_test

import (
	"context"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	itdsl "anchor/cmd/it/shared/dsl"
)

func createStartupProduct(t *testing.T) *itdsl.ProductContext {
	t.Helper()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "tenant"}).
		Product(itdsl.ProductOpts{Alias: "product", TenantAlias: "tenant"}).
		Build()
	return state.Product("product")
}

func TestDefaultWorkspaceStartupSweep(t *testing.T) {
	// Not parallel: startup reconciliation visits organizations across every product.
	ctx := context.Background()
	first := createStartupProduct(t)
	second := createStartupProduct(t)
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

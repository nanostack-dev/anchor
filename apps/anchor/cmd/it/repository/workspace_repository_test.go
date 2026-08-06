package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/workspace"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceRepositorySearchByOrganizationIDEmpty(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	result, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), productID, organizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestWorkspaceRepositorySearchByOrganizationIDPaginationBoundaries(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	const total = 5
	created := make([]string, 0, total)
	for range total {
		created = append(created, createWorkspace(t, organizationID))
	}

	// Full page in one shot.
	result, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), productID, organizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			Sort: []search.Sort[workspace.SortFieldProductWorkspace]{
				{Field: workspace.SortFieldProductWorkspaceCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), result.Total)
	assert.Equal(t, total, result.Count)
	require.Len(t, result.Items, total)
	for i, item := range result.Items {
		assert.Equal(t, created[i], item.ID)
	}

	// First page of 2: total still reflects all matches, not the page.
	page1, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), productID, organizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			Sort: []search.Sort[workspace.SortFieldProductWorkspace]{
				{Field: workspace.SortFieldProductWorkspaceCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 2, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page1.Total)
	assert.Equal(t, 2, page1.Count)
	require.Len(t, page1.Items, 2)
	assert.Equal(t, created[0], page1.Items[0].ID)
	assert.Equal(t, created[1], page1.Items[1].ID)

	// Last page: partial, offset past most rows.
	page3, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), productID, organizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			Sort: []search.Sort[workspace.SortFieldProductWorkspace]{
				{Field: workspace.SortFieldProductWorkspaceCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 2, Offset: 4},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page3.Total)
	assert.Equal(t, 1, page3.Count)
	require.Len(t, page3.Items, 1)
	assert.Equal(t, created[4], page3.Items[0].ID)

	// Offset beyond total: no rows, total still accurate.
	pageOOB, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), productID, organizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			Pagination: search.Pagination{Limit: 10, Offset: 50},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestWorkspaceRepositorySearchByOrganizationIDSortDirections(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	// Names chosen so ascending/descending name order is unambiguous.
	names := []string{"Alpha", "Bravo", "Charlie"}
	ids := make(map[string]string, len(names))
	for _, name := range names {
		entity := workspace.Workspace{OrganizationID: organizationID, Name: name}
		entity.GenerateID()
		created, err := repoCtx.WorkspaceRepository.Create(context.Background(), entity)
		require.NoError(t, err)
		ids[name] = created.ID
	}

	asc, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), productID, organizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			Sort: []search.Sort[workspace.SortFieldProductWorkspace]{
				{Field: workspace.SortFieldProductWorkspaceName, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, asc.Items, 3)
	assert.Equal(t, []string{ids["Alpha"], ids["Bravo"], ids["Charlie"]},
		[]string{asc.Items[0].ID, asc.Items[1].ID, asc.Items[2].ID})

	desc, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), productID, organizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			Sort: []search.Sort[workspace.SortFieldProductWorkspace]{
				{Field: workspace.SortFieldProductWorkspaceName, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, desc.Items, 3)
	assert.Equal(t, []string{ids["Charlie"], ids["Bravo"], ids["Alpha"]},
		[]string{desc.Items[0].ID, desc.Items[1].ID, desc.Items[2].ID})
}

func TestWorkspaceRepositorySearchByOrganizationIDFilterByIDsAndNames(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	first := createWorkspace(t, organizationID)
	second := createWorkspace(t, organizationID)
	_ = createWorkspace(t, organizationID) // unrelated third workspace, must not match

	byID, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), productID, organizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			Filter:     &workspace.SearchWorkspaceFilter{IDs: []string{first, second}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byID.Total)
	gotIDs := []string{byID.Items[0].ID, byID.Items[1].ID}
	assert.ElementsMatch(t, []string{first, second}, gotIDs)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), productID, organizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			Filter:     &workspace.SearchWorkspaceFilter{IDs: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestWorkspaceRepositorySearchByOrganizationIDFullTextSearch(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)

	entity := workspace.Workspace{OrganizationID: organizationID, Name: "Atlas Migration Workspace"}
	entity.GenerateID()
	matching, err := repoCtx.WorkspaceRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	other := workspace.Workspace{OrganizationID: organizationID, Name: "Billing Console"}
	other.GenerateID()
	_, err = repoCtx.WorkspaceRepository.Create(context.Background(), other)
	require.NoError(t, err)

	// The filter is a plain SQL LIKE (see jetx.BuildFullTextSearchFilter), not
	// tsvector — it is case-sensitive, so the term must match the stored case.
	term := "Atlas"
	result, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), productID, organizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			FullTextSearch: &term,
			Pagination:     search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, matching.ID, result.Items[0].ID)
}

func TestWorkspaceRepositorySearchByOrganizationIDScopesToOrganization(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)
	otherProductID, otherOrganizationID := tenantProductOrgChain(t)

	inScope := createWorkspace(t, organizationID)
	_ = createWorkspace(t, otherOrganizationID) // must never leak into the first org's results

	result, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), productID, organizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, inScope, result.Items[0].ID)

	otherResult, err := repoCtx.WorkspaceRepository.SearchByOrganizationID(
		context.Background(), otherProductID, otherOrganizationID,
		search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
}

package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/organization"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationRepositorySearchByProductIDEmpty(t *testing.T) {
	productID := tenantProductChain(t)

	result, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestOrganizationRepositorySearchByProductIDPaginationBoundaries(t *testing.T) {
	productID := tenantProductChain(t)

	const total = 5
	created := make([]string, 0, total)
	for range total {
		created = append(created, createOrganization(t, productID))
	}

	// Full page in one shot.
	result, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Sort: []search.Sort[organization.SortFieldProductOrganization]{
				{Field: organization.SortFieldProductOrganizationCreatedAt, Direction: search.SortAscending},
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
	page1, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Sort: []search.Sort[organization.SortFieldProductOrganization]{
				{Field: organization.SortFieldProductOrganizationCreatedAt, Direction: search.SortAscending},
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

	// Second page of 2.
	page2, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Sort: []search.Sort[organization.SortFieldProductOrganization]{
				{Field: organization.SortFieldProductOrganizationCreatedAt, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 2, Offset: 2},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page2.Total)
	assert.Equal(t, 2, page2.Count)
	require.Len(t, page2.Items, 2)
	assert.Equal(t, created[2], page2.Items[0].ID)
	assert.Equal(t, created[3], page2.Items[1].ID)

	// Last page: partial, offset past most rows.
	page3, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Sort: []search.Sort[organization.SortFieldProductOrganization]{
				{Field: organization.SortFieldProductOrganizationCreatedAt, Direction: search.SortAscending},
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
	pageOOB, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Pagination: search.Pagination{Limit: 10, Offset: 50},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestOrganizationRepositorySearchByProductIDSortDirections(t *testing.T) {
	productID := tenantProductChain(t)

	// Names chosen so ascending/descending name order is unambiguous.
	names := []string{"Alpha", "Bravo", "Charlie"}
	ids := make(map[string]string, len(names))
	for _, name := range names {
		entity := organization.Organization{ProductID: productID, Name: name}
		entity.GenerateID()
		created, err := repoCtx.OrganizationRepository.Create(context.Background(), entity)
		require.NoError(t, err)
		ids[name] = created.ID
	}

	for _, field := range []organization.SortFieldProductOrganization{
		organization.SortFieldProductOrganizationCreatedAt,
		organization.SortFieldProductOrganizationUpdatedAt,
		organization.SortFieldProductOrganizationName,
	} {
		asc, err := repoCtx.OrganizationRepository.SearchByProductID(
			context.Background(), productID,
			search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
				Sort: []search.Sort[organization.SortFieldProductOrganization]{
					{Field: field, Direction: search.SortAscending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		)
		require.NoError(t, err)
		require.Len(t, asc.Items, 3)

		desc, err := repoCtx.OrganizationRepository.SearchByProductID(
			context.Background(), productID,
			search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
				Sort: []search.Sort[organization.SortFieldProductOrganization]{
					{Field: field, Direction: search.SortDescending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		)
		require.NoError(t, err)
		require.Len(t, desc.Items, 3)

		// Descending order must be the exact reverse of ascending order.
		for i := range asc.Items {
			assert.Equal(t, asc.Items[i].ID, desc.Items[len(desc.Items)-1-i].ID)
		}
	}

	// Name field specifically must produce a deterministic, known order.
	asc, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Sort: []search.Sort[organization.SortFieldProductOrganization]{
				{Field: organization.SortFieldProductOrganizationName, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, asc.Items, 3)
	assert.Equal(t, []string{ids["Alpha"], ids["Bravo"], ids["Charlie"]},
		[]string{asc.Items[0].ID, asc.Items[1].ID, asc.Items[2].ID})

	desc, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Sort: []search.Sort[organization.SortFieldProductOrganization]{
				{Field: organization.SortFieldProductOrganizationName, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, desc.Items, 3)
	assert.Equal(t, []string{ids["Charlie"], ids["Bravo"], ids["Alpha"]},
		[]string{desc.Items[0].ID, desc.Items[1].ID, desc.Items[2].ID})
}

func TestOrganizationRepositorySearchByProductIDFilterByIDsAndNames(t *testing.T) {
	productID := tenantProductChain(t)

	first := createOrganization(t, productID)
	second := createOrganization(t, productID)
	_ = createOrganization(t, productID) // unrelated third organization, must not match

	byID, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Filter:     &organization.SearchProductOrganizationFilter{IDs: []string{first, second}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byID.Total)
	gotIDs := []string{byID.Items[0].ID, byID.Items[1].ID}
	assert.ElementsMatch(t, []string{first, second}, gotIDs)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Filter:     &organization.SearchProductOrganizationFilter{IDs: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestOrganizationRepositorySearchByProductIDFilterByNames(t *testing.T) {
	productID := tenantProductChain(t)

	target := organization.Organization{ProductID: productID, Name: uniqueName(t, "Target Org")}
	target.GenerateID()
	created, err := repoCtx.OrganizationRepository.Create(context.Background(), target)
	require.NoError(t, err)

	_ = createOrganization(t, productID) // unrelated, must not match

	byName, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Filter:     &organization.SearchProductOrganizationFilter{Names: []string{created.Name}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, byName.Items, 1)
	assert.Equal(t, created.ID, byName.Items[0].ID)

	// A name filter matching nothing returns an empty, not an error.
	none, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Filter:     &organization.SearchProductOrganizationFilter{Names: []string{"no such organization name"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestOrganizationRepositorySearchByProductIDFullTextSearch(t *testing.T) {
	productID := tenantProductChain(t)

	entity := organization.Organization{ProductID: productID, Name: "Atlas Migration Org"}
	entity.GenerateID()
	matching, err := repoCtx.OrganizationRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	other := organization.Organization{ProductID: productID, Name: "Billing Console"}
	other.GenerateID()
	_, err = repoCtx.OrganizationRepository.Create(context.Background(), other)
	require.NoError(t, err)

	// The filter is a plain SQL LIKE (see jetx.BuildFullTextSearchFilter), not
	// tsvector — it is case-sensitive, so the term must match the stored case.
	term := "Atlas"
	result, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			FullTextSearch: &term,
			Pagination:     search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, matching.ID, result.Items[0].ID)
}

func TestOrganizationRepositorySearchByProductIDScopesToProduct(t *testing.T) {
	productID := tenantProductChain(t)
	otherProductID := tenantProductChain(t)

	inScope := createOrganization(t, productID)
	_ = createOrganization(t, otherProductID) // must never leak into the first product's results

	result, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, inScope, result.Items[0].ID)

	otherResult, err := repoCtx.OrganizationRepository.SearchByProductID(
		context.Background(), otherProductID,
		search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
}

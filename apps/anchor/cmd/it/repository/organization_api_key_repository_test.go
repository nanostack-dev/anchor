package repository_test

import (
	"context"
	"testing"

	orgapikey "anchor/internal/domain/organization/apikey"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createOrganizationAPIKey inserts an organization API key fixture under
// organizationID and returns its ID.
func createOrganizationAPIKey(t *testing.T, organizationID string) string {
	t.Helper()

	entity := orgapikey.OrganizationAPIKey{
		OrganizationID:  organizationID,
		Name:            uniqueName(t, "Test API Key"),
		HashedValue:     uniqueName(t, "hashed"),
		ObfuscatedValue: uniqueName(t, "obfuscated"),
		Status:          orgapikey.StatusActive,
	}
	entity.GenerateID()

	created, err := repoCtx.OrganizationAPIKeyRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created.ID
}

func TestOrganizationAPIKeyRepositorySearchByOrganizationIDEmpty(t *testing.T) {
	_, organizationID := tenantProductOrgChain(t)

	result, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestOrganizationAPIKeyRepositorySearchByOrganizationIDPaginationBoundaries(t *testing.T) {
	_, organizationID := tenantProductOrgChain(t)

	const total = 5
	created := make([]string, 0, total)
	for range total {
		created = append(created, createOrganizationAPIKey(t, organizationID))
	}

	sortByCreatedAtAsc := []search.Sort[orgapikey.SortFieldOrganizationAPIKey]{
		{Field: orgapikey.SortFieldOrganizationAPIKeyCreatedAt, Direction: search.SortAscending},
	}

	// Full page in one shot.
	result, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Sort:       sortByCreatedAtAsc,
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), result.Total)
	assert.Equal(t, total, result.Count)
	require.Len(t, result.Items, total)
	for i, item := range result.Items {
		assert.Equal(t, created[i], item.ID)
	}

	// Page 1 of 2: total still reflects all matches, not the page.
	page1, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Sort:       sortByCreatedAtAsc,
				Pagination: search.Pagination{Limit: 2, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page1.Total)
	assert.Equal(t, 2, page1.Count)
	require.Len(t, page1.Items, 2)
	assert.Equal(t, created[0], page1.Items[0].ID)
	assert.Equal(t, created[1], page1.Items[1].ID)

	// Page 2 of 2.
	page2, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Sort:       sortByCreatedAtAsc,
				Pagination: search.Pagination{Limit: 2, Offset: 2},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page2.Total)
	assert.Equal(t, 2, page2.Count)
	require.Len(t, page2.Items, 2)
	assert.Equal(t, created[2], page2.Items[0].ID)
	assert.Equal(t, created[3], page2.Items[1].ID)

	// Page 3 of 2: partial, offset past most rows.
	page3, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Sort:       sortByCreatedAtAsc,
				Pagination: search.Pagination{Limit: 2, Offset: 4},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page3.Total)
	assert.Equal(t, 1, page3.Count)
	require.Len(t, page3.Items, 1)
	assert.Equal(t, created[4], page3.Items[0].ID)

	// Offset beyond total: no rows, total still accurate.
	pageOOB, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Pagination: search.Pagination{Limit: 10, Offset: 50},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestOrganizationAPIKeyRepositorySearchByOrganizationIDSortDirections(t *testing.T) {
	_, organizationID := tenantProductOrgChain(t)

	// One API key per case, distinguishable by name/status/last-used so every
	// declared sort field produces an unambiguous ascending/descending order.
	first := orgapikey.OrganizationAPIKey{
		OrganizationID:  organizationID,
		Name:            "Alpha " + uniqueName(t, "key"),
		HashedValue:     uniqueName(t, "hashed"),
		ObfuscatedValue: uniqueName(t, "obfuscated"),
		Status:          orgapikey.StatusActive,
	}
	first.GenerateID()
	createdFirst, err := repoCtx.OrganizationAPIKeyRepository.Create(context.Background(), first)
	require.NoError(t, err)

	second := orgapikey.OrganizationAPIKey{
		OrganizationID:  organizationID,
		Name:            "Bravo " + uniqueName(t, "key"),
		HashedValue:     uniqueName(t, "hashed"),
		ObfuscatedValue: uniqueName(t, "obfuscated"),
		Status:          orgapikey.StatusInactive,
	}
	second.GenerateID()
	createdSecond, err := repoCtx.OrganizationAPIKeyRepository.Create(context.Background(), second)
	require.NoError(t, err)

	require.NoError(t, repoCtx.OrganizationAPIKeyRepository.UpdateLastUsedAt(
		context.Background(), organizationID, createdSecond.ID,
	))

	fields := []orgapikey.SortFieldOrganizationAPIKey{
		orgapikey.SortFieldOrganizationAPIKeyID,
		orgapikey.SortFieldOrganizationAPIKeyCreatedAt,
		orgapikey.SortFieldOrganizationAPIKeyUpdatedAt,
		orgapikey.SortFieldOrganizationAPIKeyName,
		orgapikey.SortFieldOrganizationAPIKeyStatus,
		orgapikey.SortFieldOrganizationAPIKeyLastUsed,
	}

	for _, field := range fields {
		t.Run(string(field), func(t *testing.T) {
			asc, ascErr := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
				context.Background(),
				orgapikey.SearchOrganizationAPIKeysInput{
					OrganizationID: organizationID,
					Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
						Sort: []search.Sort[orgapikey.SortFieldOrganizationAPIKey]{
							{Field: field, Direction: search.SortAscending},
						},
						Pagination: search.Pagination{Limit: 10, Offset: 0},
					},
				},
			)
			require.NoError(t, ascErr)
			require.Len(t, asc.Items, 2)

			desc, descErr := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
				context.Background(),
				orgapikey.SearchOrganizationAPIKeysInput{
					OrganizationID: organizationID,
					Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
						Sort: []search.Sort[orgapikey.SortFieldOrganizationAPIKey]{
							{Field: field, Direction: search.SortDescending},
						},
						Pagination: search.Pagination{Limit: 10, Offset: 0},
					},
				},
			)
			require.NoError(t, descErr)
			require.Len(t, desc.Items, 2)

			// Ascending and descending must be exact reverses of one another,
			// and together must contain exactly the two fixtures.
			assert.Equal(t, asc.Items[0].ID, desc.Items[1].ID)
			assert.Equal(t, asc.Items[1].ID, desc.Items[0].ID)
			assert.ElementsMatch(t,
				[]string{createdFirst.ID, createdSecond.ID},
				[]string{asc.Items[0].ID, asc.Items[1].ID},
			)
		})
	}
}

func TestOrganizationAPIKeyRepositorySearchByOrganizationIDFilterByIDs(t *testing.T) {
	_, organizationID := tenantProductOrgChain(t)

	first := createOrganizationAPIKey(t, organizationID)
	second := createOrganizationAPIKey(t, organizationID)
	_ = createOrganizationAPIKey(t, organizationID) // unrelated third key, must not match

	byID, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Filter: &orgapikey.SearchOrganizationAPIKeyFilter{
					OrganizationAPIKeyIDs: []string{first, second},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byID.Total)
	gotIDs := []string{byID.Items[0].ID, byID.Items[1].ID}
	assert.ElementsMatch(t, []string{first, second}, gotIDs)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Filter: &orgapikey.SearchOrganizationAPIKeyFilter{
					OrganizationAPIKeyIDs: []string{"does-not-exist"},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestOrganizationAPIKeyRepositorySearchByOrganizationIDFilterByNames(t *testing.T) {
	_, organizationID := tenantProductOrgChain(t)

	matchingName := uniqueName(t, "Matching Key")
	matching := orgapikey.OrganizationAPIKey{
		OrganizationID:  organizationID,
		Name:            matchingName,
		HashedValue:     uniqueName(t, "hashed"),
		ObfuscatedValue: uniqueName(t, "obfuscated"),
		Status:          orgapikey.StatusActive,
	}
	matching.GenerateID()
	createdMatching, err := repoCtx.OrganizationAPIKeyRepository.Create(context.Background(), matching)
	require.NoError(t, err)

	_ = createOrganizationAPIKey(t, organizationID) // unrelated key, must not match

	byName, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Filter:     &orgapikey.SearchOrganizationAPIKeyFilter{Names: []string{matchingName}},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, byName.Items, 1)
	assert.Equal(t, createdMatching.ID, byName.Items[0].ID)

	none, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Filter:     &orgapikey.SearchOrganizationAPIKeyFilter{Names: []string{"no such name"}},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestOrganizationAPIKeyRepositorySearchByOrganizationIDFilterByStatus(t *testing.T) {
	_, organizationID := tenantProductOrgChain(t)

	active := createOrganizationAPIKey(t, organizationID)

	inactiveEntity := orgapikey.OrganizationAPIKey{
		OrganizationID:  organizationID,
		Name:            uniqueName(t, "Inactive Key"),
		HashedValue:     uniqueName(t, "hashed"),
		ObfuscatedValue: uniqueName(t, "obfuscated"),
		Status:          orgapikey.StatusActive,
	}
	inactiveEntity.GenerateID()
	createdInactive, err := repoCtx.OrganizationAPIKeyRepository.Create(context.Background(), inactiveEntity)
	require.NoError(t, err)
	require.NoError(t, repoCtx.OrganizationAPIKeyRepository.UpdateStatus(
		context.Background(), organizationID, createdInactive.ID, orgapikey.StatusInactive,
	))

	activeResult, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Filter:     &orgapikey.SearchOrganizationAPIKeyFilter{Status: []string{string(orgapikey.StatusActive)}},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, activeResult.Items, 1)
	assert.Equal(t, active, activeResult.Items[0].ID)

	inactiveResult, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Filter: &orgapikey.SearchOrganizationAPIKeyFilter{
					Status: []string{string(orgapikey.StatusInactive)},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, inactiveResult.Items, 1)
	assert.Equal(t, createdInactive.ID, inactiveResult.Items[0].ID)
}

func TestOrganizationAPIKeyRepositorySearchByOrganizationIDFullTextSearch(t *testing.T) {
	_, organizationID := tenantProductOrgChain(t)

	entity := orgapikey.OrganizationAPIKey{
		OrganizationID:  organizationID,
		Name:            "Atlas Migration Key",
		HashedValue:     uniqueName(t, "hashed"),
		ObfuscatedValue: uniqueName(t, "obfuscated"),
		Status:          orgapikey.StatusActive,
	}
	entity.GenerateID()
	matching, err := repoCtx.OrganizationAPIKeyRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	other := orgapikey.OrganizationAPIKey{
		OrganizationID:  organizationID,
		Name:            "Billing Console Key",
		HashedValue:     uniqueName(t, "hashed"),
		ObfuscatedValue: uniqueName(t, "obfuscated"),
		Status:          orgapikey.StatusActive,
	}
	other.GenerateID()
	_, err = repoCtx.OrganizationAPIKeyRepository.Create(context.Background(), other)
	require.NoError(t, err)

	// The filter is a plain SQL LIKE (see jetx.BuildFullTextSearchFilter), not
	// tsvector — it is case-sensitive, so the term must match the stored case.
	term := "Atlas"
	result, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				FullTextSearch: &term,
				Pagination:     search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, matching.ID, result.Items[0].ID)
}

func TestOrganizationAPIKeyRepositorySearchByOrganizationIDScopesToOrganization(t *testing.T) {
	_, organizationID := tenantProductOrgChain(t)
	_, otherOrganizationID := tenantProductOrgChain(t)

	inScope := createOrganizationAPIKey(t, organizationID)
	_ = createOrganizationAPIKey(t, otherOrganizationID) // must never leak into the first org's results

	result, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: organizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, inScope, result.Items[0].ID)

	otherResult, err := repoCtx.OrganizationAPIKeyRepository.SearchByOrganizationID(
		context.Background(),
		orgapikey.SearchOrganizationAPIKeysInput{
			OrganizationID: otherOrganizationID,
			Request: search.Request[orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey]{
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
}

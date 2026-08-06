package repository_test

import (
	"context"
	"testing"
	"time"

	"anchor/internal/domain/product/apikey"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createProductAPIKey inserts a product API key fixture under productID and
// returns the created domain entity. Callers may override Name/Status/LastUsedAt
// before Create to shape sort/filter test data; HashedValue/ObfuscatedValue are
// always unique to satisfy the entity's natural identity even though the schema
// itself only enforces (product_id, name) uniqueness.
func createProductAPIKey(t *testing.T, productID string, mutate func(*apikey.ProductAPIKey)) apikey.ProductAPIKey {
	t.Helper()

	entity := apikey.ProductAPIKey{
		ProductID:       productID,
		Name:            uniqueName(t, "Test API Key"),
		HashedValue:     uniqueName(t, "hashed"),
		ObfuscatedValue: uniqueName(t, "obfuscated"),
		Status:          apikey.StatusActive,
	}
	entity.GenerateID()

	if mutate != nil {
		mutate(&entity)
	}

	created, err := repoCtx.ProductAPIKeyRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created
}

func TestProductAPIKeyRepositorySearchByProductIDEmpty(t *testing.T) {
	productID := tenantProductChain(t)

	result, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestProductAPIKeyRepositorySearchByProductIDPaginationBoundaries(t *testing.T) {
	productID := tenantProductChain(t)

	const total = 5
	created := make([]string, 0, total)
	for range total {
		key := createProductAPIKey(t, productID, nil)
		created = append(created, key.ID)
	}

	sortByCreatedAtAsc := []search.Sort[apikey.SortFieldProductAPIKey]{
		{Field: apikey.SortFieldProductAPIKeyCreatedAt, Direction: search.SortAscending},
	}

	// Full page in one shot.
	result, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
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

	// First page of 2: total still reflects all matches, not the page.
	page1, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
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

	// Second page of 2.
	page2, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
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

	// Last page: partial, offset past most rows.
	page3, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
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
	pageOOB, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Pagination: search.Pagination{Limit: 10, Offset: 50},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestProductAPIKeyRepositorySearchByProductIDSortByName(t *testing.T) {
	productID := tenantProductChain(t)

	names := []string{"Alpha", "Bravo", "Charlie"}
	ids := make(map[string]string, len(names))
	for _, name := range names {
		key := createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
			a.Name = name
		})
		ids[name] = key.ID
	}

	asc, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Sort: []search.Sort[apikey.SortFieldProductAPIKey]{
					{Field: apikey.SortFieldProductAPIKeyName, Direction: search.SortAscending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, asc.Items, 3)
	assert.Equal(t, []string{ids["Alpha"], ids["Bravo"], ids["Charlie"]},
		[]string{asc.Items[0].ID, asc.Items[1].ID, asc.Items[2].ID})

	desc, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Sort: []search.Sort[apikey.SortFieldProductAPIKey]{
					{Field: apikey.SortFieldProductAPIKeyName, Direction: search.SortDescending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, desc.Items, 3)
	assert.Equal(t, []string{ids["Charlie"], ids["Bravo"], ids["Alpha"]},
		[]string{desc.Items[0].ID, desc.Items[1].ID, desc.Items[2].ID})
}

func TestProductAPIKeyRepositorySearchByProductIDSortByCreatedAt(t *testing.T) {
	productID := tenantProductChain(t)

	first := createProductAPIKey(t, productID, nil)
	second := createProductAPIKey(t, productID, nil)
	third := createProductAPIKey(t, productID, nil)

	asc, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Sort: []search.Sort[apikey.SortFieldProductAPIKey]{
					{Field: apikey.SortFieldProductAPIKeyCreatedAt, Direction: search.SortAscending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, asc.Items, 3)
	assert.Equal(t, []string{first.ID, second.ID, third.ID},
		[]string{asc.Items[0].ID, asc.Items[1].ID, asc.Items[2].ID})

	desc, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Sort: []search.Sort[apikey.SortFieldProductAPIKey]{
					{Field: apikey.SortFieldProductAPIKeyCreatedAt, Direction: search.SortDescending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, desc.Items, 3)
	assert.Equal(t, []string{third.ID, second.ID, first.ID},
		[]string{desc.Items[0].ID, desc.Items[1].ID, desc.Items[2].ID})
}

func TestProductAPIKeyRepositorySearchByProductIDSortByUpdatedAt(t *testing.T) {
	productID := tenantProductChain(t)

	a := createProductAPIKey(t, productID, nil)
	b := createProductAPIKey(t, productID, nil)
	c := createProductAPIKey(t, productID, nil)

	// Touch in an order distinct from creation order (b, a, c) so a sort by
	// updated_at is provably different from one by created_at.
	for _, key := range []apikey.ProductAPIKey{b, a, c} {
		_, err := repoCtx.ProductAPIKeyRepository.Update(context.Background(), key)
		require.NoError(t, err)
		time.Sleep(5 * time.Millisecond)
	}

	asc, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Sort: []search.Sort[apikey.SortFieldProductAPIKey]{
					{Field: apikey.SortFieldProductAPIKeyUpdatedAt, Direction: search.SortAscending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, asc.Items, 3)
	assert.Equal(t, []string{b.ID, a.ID, c.ID},
		[]string{asc.Items[0].ID, asc.Items[1].ID, asc.Items[2].ID})

	desc, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Sort: []search.Sort[apikey.SortFieldProductAPIKey]{
					{Field: apikey.SortFieldProductAPIKeyUpdatedAt, Direction: search.SortDescending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, desc.Items, 3)
	assert.Equal(t, []string{c.ID, a.ID, b.ID},
		[]string{desc.Items[0].ID, desc.Items[1].ID, desc.Items[2].ID})
}

func TestProductAPIKeyRepositorySearchByProductIDSortByStatus(t *testing.T) {
	productID := tenantProductChain(t)

	// ACTIVE < INACTIVE lexicographically, so pick names that pin a
	// deterministic ascending order distinct from creation order.
	active := createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.Status = apikey.StatusActive
	})
	inactive := createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.Status = apikey.StatusInactive
	})

	asc, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Sort: []search.Sort[apikey.SortFieldProductAPIKey]{
					{Field: apikey.SortFieldProductAPIKeyStatus, Direction: search.SortAscending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, asc.Items, 2)
	assert.Equal(t, []string{active.ID, inactive.ID},
		[]string{asc.Items[0].ID, asc.Items[1].ID})

	desc, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Sort: []search.Sort[apikey.SortFieldProductAPIKey]{
					{Field: apikey.SortFieldProductAPIKeyStatus, Direction: search.SortDescending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, desc.Items, 2)
	assert.Equal(t, []string{inactive.ID, active.ID},
		[]string{desc.Items[0].ID, desc.Items[1].ID})
}

func TestProductAPIKeyRepositorySearchByProductIDSortByLastUsed(t *testing.T) {
	productID := tenantProductChain(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	earliest := now.Add(-2 * time.Hour)
	middle := now.Add(-1 * time.Hour)
	latest := now

	oldest := createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.LastUsedAt = &earliest
	})
	mid := createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.LastUsedAt = &middle
	})
	newest := createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.LastUsedAt = &latest
	})

	asc, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Sort: []search.Sort[apikey.SortFieldProductAPIKey]{
					{Field: apikey.SortFieldProductAPIKeyLastUsed, Direction: search.SortAscending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, asc.Items, 3)
	assert.Equal(t, []string{oldest.ID, mid.ID, newest.ID},
		[]string{asc.Items[0].ID, asc.Items[1].ID, asc.Items[2].ID})

	desc, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Sort: []search.Sort[apikey.SortFieldProductAPIKey]{
					{Field: apikey.SortFieldProductAPIKeyLastUsed, Direction: search.SortDescending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, desc.Items, 3)
	assert.Equal(t, []string{newest.ID, mid.ID, oldest.ID},
		[]string{desc.Items[0].ID, desc.Items[1].ID, desc.Items[2].ID})
}

func TestProductAPIKeyRepositorySearchByProductIDFilterByIDs(t *testing.T) {
	productID := tenantProductChain(t)

	first := createProductAPIKey(t, productID, nil)
	second := createProductAPIKey(t, productID, nil)
	_ = createProductAPIKey(t, productID, nil) // unrelated third key, must not match

	result, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Filter:     &apikey.SearchProductAPIKeyFilter{ProductAPIKeyIDs: []string{first.ID, second.ID}},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)
	gotIDs := []string{result.Items[0].ID, result.Items[1].ID}
	assert.ElementsMatch(t, []string{first.ID, second.ID}, gotIDs)
}

func TestProductAPIKeyRepositorySearchByProductIDFilterByNames(t *testing.T) {
	productID := tenantProductChain(t)

	target := createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.Name = uniqueName(t, "Named Target")
	})
	_ = createProductAPIKey(t, productID, nil) // unrelated, must not match

	result, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Filter:     &apikey.SearchProductAPIKeyFilter{Names: []string{target.Name}},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, target.ID, result.Items[0].ID)
}

func TestProductAPIKeyRepositorySearchByProductIDFilterByStatus(t *testing.T) {
	productID := tenantProductChain(t)

	active := createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.Status = apikey.StatusActive
	})
	_ = createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.Status = apikey.StatusInactive
	})

	result, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Filter:     &apikey.SearchProductAPIKeyFilter{Status: []string{string(apikey.StatusActive)}},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, active.ID, result.Items[0].ID)
}

func TestProductAPIKeyRepositorySearchByProductIDFilterByLastUsedBeforeAndAfter(t *testing.T) {
	productID := tenantProductChain(t)

	now := time.Now().UTC().Truncate(time.Millisecond)
	old := now.Add(-48 * time.Hour)
	recent := now.Add(-1 * time.Hour)

	oldKey := createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.LastUsedAt = &old
	})
	recentKey := createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.LastUsedAt = &recent
	})

	cutoff := now.Add(-24 * time.Hour)

	before, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Filter:     &apikey.SearchProductAPIKeyFilter{LastUsedBefore: &cutoff},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, before.Items, 1)
	assert.Equal(t, oldKey.ID, before.Items[0].ID)

	after, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Filter:     &apikey.SearchProductAPIKeyFilter{LastUsedAfter: &cutoff},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, after.Items, 1)
	assert.Equal(t, recentKey.ID, after.Items[0].ID)
}

func TestProductAPIKeyRepositorySearchByProductIDFilterMatchesNothing(t *testing.T) {
	productID := tenantProductChain(t)

	_ = createProductAPIKey(t, productID, nil)

	result, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Filter:     &apikey.SearchProductAPIKeyFilter{ProductAPIKeyIDs: []string{"does-not-exist"}},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Empty(t, result.Items)
}

func TestProductAPIKeyRepositorySearchByProductIDFullTextSearch(t *testing.T) {
	productID := tenantProductChain(t)

	matching := createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.Name = uniqueName(t, "Atlas Migration Key")
	})
	_ = createProductAPIKey(t, productID, func(a *apikey.ProductAPIKey) {
		a.Name = uniqueName(t, "Billing Console Key")
	})

	// applyFullTextSearch is a plain SQL LIKE (not tsvector) — it is
	// case-sensitive, so the term must match the stored case exactly.
	term := "Atlas"
	result, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				FullTextSearch: &term,
				Pagination:     search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, matching.ID, result.Items[0].ID)
}

func TestProductAPIKeyRepositorySearchByProductIDScopesToProduct(t *testing.T) {
	productID := tenantProductChain(t)
	otherProductID := tenantProductChain(t)

	inScope := createProductAPIKey(t, productID, nil)
	_ = createProductAPIKey(t, otherProductID, nil) // must never leak into the first product's results

	result, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: productID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, inScope.ID, result.Items[0].ID)

	otherResult, err := repoCtx.ProductAPIKeyRepository.SearchByProductID(
		context.Background(),
		apikey.SearchProductAPIKeysInput{
			ProductID: otherProductID,
			Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
	for _, item := range otherResult.Items {
		assert.NotEqual(t, inScope.ID, item.ID)
	}
}

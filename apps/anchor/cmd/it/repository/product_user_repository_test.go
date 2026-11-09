package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/product/user"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createProductUserWithDetails inserts a product user fixture with caller-chosen
// name/email/status and returns the full created entity, for tests that need to
// assert on those fields (filters, sorting, full-text search).
func createProductUserWithDetails(t *testing.T, entity user.ProductUser) user.ProductUser {
	t.Helper()
	entity.GenerateID()

	created, err := repoCtx.ProductUserRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created
}

func TestProductUserRepositorySearchByProductIDEmpty(t *testing.T) {
	productID := tenantProductChain(t)

	result, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestProductUserRepositorySearchByProductIDPaginationBoundaries(t *testing.T) {
	productID := tenantProductChain(t)

	const total = 5
	created := make([]string, 0, total)
	for range total {
		created = append(created, createProductUser(t, productID))
	}

	sortByCreatedAt := []search.Sort[user.SortFieldProductUser]{
		{Field: user.SortFieldProductUserCreatedAt, Direction: search.SortAscending},
	}

	// Full page in one shot.
	result, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Sort:       sortByCreatedAt,
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

	// Page 1 of 2: total still reflects all matches, not the page.
	page1, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Sort:       sortByCreatedAt,
			Pagination: search.Pagination{Limit: 2, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page1.Total)
	assert.Equal(t, 2, page1.Count)
	require.Len(t, page1.Items, 2)
	assert.Equal(t, created[0], page1.Items[0].ID)
	assert.Equal(t, created[1], page1.Items[1].ID)

	// Page 2 of 2.
	page2, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Sort:       sortByCreatedAt,
			Pagination: search.Pagination{Limit: 2, Offset: 2},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page2.Total)
	assert.Equal(t, 2, page2.Count)
	require.Len(t, page2.Items, 2)
	assert.Equal(t, created[2], page2.Items[0].ID)
	assert.Equal(t, created[3], page2.Items[1].ID)

	// Page 3: partial, offset past most rows.
	page3, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Sort:       sortByCreatedAt,
			Pagination: search.Pagination{Limit: 2, Offset: 4},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), page3.Total)
	assert.Equal(t, 1, page3.Count)
	require.Len(t, page3.Items, 1)
	assert.Equal(t, created[4], page3.Items[0].ID)

	// Offset beyond total: no rows, total still accurate.
	pageOOB, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Pagination: search.Pagination{Limit: 10, Offset: 50},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestProductUserRepositorySearchByProductIDSortDirections(t *testing.T) {
	productID := tenantProductChain(t)

	alpha := createProductUserWithDetails(t, user.ProductUser{
		ProductID: productID,
		Name:      "Alpha",
		Email:     "alpha@example.com",
		Status:    user.ProductUserStatusActive,
	})
	bravo := createProductUserWithDetails(t, user.ProductUser{
		ProductID: productID,
		Name:      "Bravo",
		Email:     "bravo@example.com",
		Status:    user.ProductUserStatusSuspended,
	})
	charlie := createProductUserWithDetails(t, user.ProductUser{
		ProductID: productID,
		Name:      "Charlie",
		Email:     "charlie@example.com",
		Status:    user.ProductUserStatusActive,
	})

	// Touch alpha after every fixture is created so its UpdatedAt sorts last,
	// distinct from insertion (CreatedAt) order.
	_, updateErr := repoCtx.ProductUserRepository.Update(
		context.Background(), productID, alpha.ID,
		user.ProductUser{Email: alpha.Email, Name: alpha.Name, Status: alpha.Status},
	)
	require.NoError(t, updateErr)

	assertOrder := func(
		t *testing.T,
		field user.SortFieldProductUser,
		direction search.SortDirection,
		expectedIDs []string,
	) {
		t.Helper()

		result, err := repoCtx.ProductUserRepository.SearchByProductID(
			context.Background(), productID,
			search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
				Sort:       []search.Sort[user.SortFieldProductUser]{{Field: field, Direction: direction}},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		)
		require.NoError(t, err)
		require.Len(t, result.Items, len(expectedIDs))

		gotIDs := make([]string, len(result.Items))
		for i, item := range result.Items {
			gotIDs[i] = item.ID
		}
		assert.Equal(t, expectedIDs, gotIDs)
	}

	t.Run("Name", func(t *testing.T) {
		assertOrder(t, user.SortFieldProductUserName, search.SortAscending,
			[]string{alpha.ID, bravo.ID, charlie.ID})
		assertOrder(t, user.SortFieldProductUserName, search.SortDescending,
			[]string{charlie.ID, bravo.ID, alpha.ID})
	})

	t.Run("Email", func(t *testing.T) {
		assertOrder(t, user.SortFieldProductUserEmail, search.SortAscending,
			[]string{alpha.ID, bravo.ID, charlie.ID})
		assertOrder(t, user.SortFieldProductUserEmail, search.SortDescending,
			[]string{charlie.ID, bravo.ID, alpha.ID})
	})

	t.Run("CreatedAt", func(t *testing.T) {
		assertOrder(t, user.SortFieldProductUserCreatedAt, search.SortAscending,
			[]string{alpha.ID, bravo.ID, charlie.ID})
		assertOrder(t, user.SortFieldProductUserCreatedAt, search.SortDescending,
			[]string{charlie.ID, bravo.ID, alpha.ID})
	})

	t.Run("UpdatedAt", func(t *testing.T) {
		// alpha was updated after all three inserts, so it now sorts last ascending.
		assertOrder(t, user.SortFieldProductUserUpdatedAt, search.SortAscending,
			[]string{bravo.ID, charlie.ID, alpha.ID})
		assertOrder(t, user.SortFieldProductUserUpdatedAt, search.SortDescending,
			[]string{alpha.ID, charlie.ID, bravo.ID})
	})

	t.Run("Status", func(t *testing.T) {
		// ACTIVE < INACTIVE alphabetically. alpha and charlie are both ACTIVE, so
		// their relative order within that bucket isn't asserted, only membership.
		asc, err := repoCtx.ProductUserRepository.SearchByProductID(
			context.Background(), productID,
			search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
				Sort: []search.Sort[user.SortFieldProductUser]{
					{Field: user.SortFieldProductUserStatus, Direction: search.SortAscending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		)
		require.NoError(t, err)
		require.Len(t, asc.Items, 3)
		assert.ElementsMatch(t, []string{alpha.ID, charlie.ID}, []string{asc.Items[0].ID, asc.Items[1].ID})
		assert.Equal(t, bravo.ID, asc.Items[2].ID)

		desc, err := repoCtx.ProductUserRepository.SearchByProductID(
			context.Background(), productID,
			search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
				Sort: []search.Sort[user.SortFieldProductUser]{
					{Field: user.SortFieldProductUserStatus, Direction: search.SortDescending},
				},
				Pagination: search.Pagination{Limit: 10, Offset: 0},
			},
		)
		require.NoError(t, err)
		require.Len(t, desc.Items, 3)
		assert.Equal(t, bravo.ID, desc.Items[0].ID)
		assert.ElementsMatch(t, []string{alpha.ID, charlie.ID}, []string{desc.Items[1].ID, desc.Items[2].ID})
	})
}

func TestProductUserRepositorySearchByProductIDFilterByIDs(t *testing.T) {
	productID := tenantProductChain(t)

	first := createProductUser(t, productID)
	second := createProductUser(t, productID)
	_ = createProductUser(t, productID) // unrelated third, must not match

	result, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Filter:     &user.SearchProductUserFilter{IDs: []string{first, second}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)
	require.Len(t, result.Items, 2)
	assert.ElementsMatch(t, []string{first, second}, []string{result.Items[0].ID, result.Items[1].ID})

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Filter:     &user.SearchProductUserFilter{IDs: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestProductUserRepositorySearchByProductIDFilterByEmails(t *testing.T) {
	productID := tenantProductChain(t)

	target := createProductUserWithDetails(t, user.ProductUser{
		ProductID: productID,
		Name:      uniqueName(t, "Target"),
		Email:     uniqueName(t, "target") + "@example.com",
		Status:    user.ProductUserStatusActive,
	})
	_ = createProductUser(t, productID) // unrelated, must not match

	result, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Filter:     &user.SearchProductUserFilter{Emails: []string{target.Email}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, target.ID, result.Items[0].ID)

	none, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Filter:     &user.SearchProductUserFilter{Emails: []string{"nobody-here@example.com"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

// searchProductUsersByNames runs a product-user search with only the names
// filter and returns the IDs it finds.
func searchProductUsersByNames(t *testing.T, productID string, names ...string) []string {
	t.Helper()

	result, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Filter:     &user.SearchProductUserFilter{Names: names},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, int(result.Total))

	ids := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

// The spec documents the names filter as "contains any of these terms", each a
// case-insensitive, literal substring, the rule every free-text filter follows.
func TestProductUserRepositorySearchByProductIDFilterByNames(t *testing.T) {
	productID := tenantProductChain(t)

	create := func(name, email string) string {
		return createProductUserWithDetails(t, user.ProductUser{
			ProductID: productID,
			Name:      name,
			Email:     uniqueName(t, email) + "@example.com",
			Status:    user.ProductUserStatusActive,
		}).ID
	}
	joanne := create("Joanne Atlas", "joanne")
	ann := create("Ann Borealis", "ann")
	percent := create("100% Renewable", "percent")
	noPercent := create("1000 Renewable", "nopercent")
	underscore := create("seat_cap owner", "underscore")
	noUnderscore := create("seatXcap owner", "nounderscore")
	// The email holds the term; the names filter reads the name only.
	byEmail := create("Someone Else", "atlas-in-email")

	tests := map[string]struct {
		names []string
		want  []string
	}{
		"a whole name still matches":            {names: []string{"Joanne Atlas"}, want: []string{joanne}},
		"a part of the name matches":            {names: []string{"atla"}, want: []string{joanne}},
		"letter case is ignored":                {names: []string{"JOANNE atlas"}, want: []string{joanne}},
		"a term matches inside a longer name":   {names: []string{"ann"}, want: []string{joanne, ann}},
		"several terms match any of them":       {names: []string{"atlas", "borealis"}, want: []string{joanne, ann}},
		"percent matches itself":                {names: []string{"100%"}, want: []string{percent}},
		"underscore matches itself":             {names: []string{"seat_cap"}, want: []string{underscore}},
		"a term found only in the email misses": {names: []string{"atlas-in-email"}, want: []string{}},
		"a term no name holds matches nothing":  {names: []string{"No Such Name At All"}, want: []string{}},
		// The near misses exist, so the misses above come from the filter.
		"the near misses match their own names": {
			names: []string{"1000 renewable", "seatxcap", "someone else"},
			want:  []string{noPercent, noUnderscore, byEmail},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.ElementsMatch(t, tc.want, searchProductUsersByNames(t, productID, tc.names...))
		})
	}
}

func TestProductUserRepositorySearchByProductIDFilterByStatuses(t *testing.T) {
	productID := tenantProductChain(t)

	active := createProductUserWithDetails(t, user.ProductUser{
		ProductID: productID,
		Name:      uniqueName(t, "Active User"),
		Email:     uniqueName(t, "active") + "@example.com",
		Status:    user.ProductUserStatusActive,
	})
	_ = createProductUserWithDetails(t, user.ProductUser{
		ProductID: productID,
		Name:      uniqueName(t, "Inactive User"),
		Email:     uniqueName(t, "inactive") + "@example.com",
		Status:    user.ProductUserStatusSuspended,
	})

	result, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Filter:     &user.SearchProductUserFilter{Statuses: []user.ProductUserStatus{user.ProductUserStatusActive}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, active.ID, result.Items[0].ID)
	assert.Equal(t, user.ProductUserStatusActive, result.Items[0].Status)
}

func TestProductUserRepositorySearchByProductIDFilterByExternalIDs(t *testing.T) {
	productID := tenantProductChain(t)

	externalID := uniqueName(t, "ext-id")
	target := createProductUserWithDetails(t, user.ProductUser{
		ProductID:  productID,
		Name:       uniqueName(t, "External User"),
		Email:      uniqueName(t, "externalfilter") + "@example.com",
		Status:     user.ProductUserStatusActive,
		ExternalID: &externalID,
	})
	_ = createProductUser(t, productID) // unrelated, no external ID, must not match

	result, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Filter:     &user.SearchProductUserFilter{ExternalIDs: []string{externalID}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, target.ID, result.Items[0].ID)

	none, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Filter:     &user.SearchProductUserFilter{ExternalIDs: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestProductUserRepositorySearchByProductIDFullTextSearch(t *testing.T) {
	productID := tenantProductChain(t)

	matching := createProductUserWithDetails(t, user.ProductUser{
		ProductID: productID,
		Name:      "Atlas Migration User " + uniqueName(t, "ft"),
		Email:     uniqueName(t, "atlas") + "@example.com",
		Status:    user.ProductUserStatusActive,
	})
	_ = createProductUserWithDetails(t, user.ProductUser{
		ProductID: productID,
		Name:      "Billing Console User " + uniqueName(t, "ft"),
		Email:     uniqueName(t, "billing") + "@example.com",
		Status:    user.ProductUserStatusActive,
	})

	// Search ignores case, so a lower-case term finds the capitalised record.
	term := "atlas"
	result, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			FullTextSearch: &term,
			Pagination:     search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, matching.ID, result.Items[0].ID)
}

func TestProductUserRepositorySearchByProductIDScopesToProduct(t *testing.T) {
	productID := tenantProductChain(t)
	otherProductID := tenantProductChain(t)

	inScope := createProductUser(t, productID)
	_ = createProductUser(t, otherProductID) // must never leak into the first product's results

	result, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, inScope, result.Items[0].ID)

	otherResult, err := repoCtx.ProductUserRepository.SearchByProductID(
		context.Background(), otherProductID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
}

// TestProductUserRepositoryReadsStoredStatuses covers rows that the previous
// release wrote: ACTIVE reads back as active, and a value outside the contract
// enum fails the read instead of reaching a response.
func TestProductUserRepositoryReadsStoredStatuses(t *testing.T) {
	productID := tenantProductChain(t)
	ctx := context.Background()

	setStatus := func(id, status string) {
		t.Helper()
		_, err := repoCtx.DB.ExecContext(ctx, "UPDATE product_users SET status = $1 WHERE id = $2", status, id)
		require.NoError(t, err)
	}

	legacyID := createProductUser(t, productID)
	setStatus(legacyID, "ACTIVE")
	legacy, err := repoCtx.ProductUserRepository.FindByProductIDAndID(ctx, productID, legacyID)
	require.NoError(t, err)
	require.True(t, legacy.IsPresent())
	assert.Equal(t, user.ProductUserStatusActive, legacy.Value().Status, "a legacy ACTIVE row reads as active")

	unknownID := createProductUser(t, productID)
	setStatus(unknownID, "banana")
	_, err = repoCtx.ProductUserRepository.FindByProductIDAndID(ctx, productID, unknownID)
	require.ErrorIs(t, err, user.ErrUnknownProductUserStatus, "find rejects an unknown stored status")
	_, err = repoCtx.ProductUserRepository.FindByProductID(ctx, productID)
	require.ErrorIs(t, err, user.ErrUnknownProductUserStatus, "list rejects an unknown stored status")
	_, err = repoCtx.ProductUserRepository.SearchByProductID(ctx, productID,
		search.Request[user.SearchProductUserFilter, user.SortFieldProductUser]{
			Pagination: search.Pagination{Limit: 10},
		})
	require.ErrorIs(t, err, user.ErrUnknownProductUserStatus, "search rejects an unknown stored status")
}

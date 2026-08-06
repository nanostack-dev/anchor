package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/invitation"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createInvitation inserts a platform invitation fixture under tenantID and
// returns the created entity. email/code default to unique values derived
// from t's name so parallel fixtures never collide with the
// (platform_tenant_id, email) unique index.
func createInvitation(t *testing.T, tenantID string) invitation.PlatformInvitation {
	t.Helper()

	entity := invitation.PlatformInvitation{
		PlatformTenantID: tenantID,
		Email:            uniqueName(t, "invite") + "@example.com",
		Code:             uniqueName(t, "code"),
	}
	entity.GenerateID()

	created, err := repoCtx.InvitationRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created
}

func TestPlatformInvitationRepositorySearchByTenantIDEmpty(t *testing.T) {
	tenantID := createTenant(t)

	result, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestPlatformInvitationRepositorySearchByTenantIDPaginationBoundaries(t *testing.T) {
	tenantID := createTenant(t)

	const total = 5
	created := make([]string, 0, total)
	for range total {
		created = append(created, createInvitation(t, tenantID).ID)
	}

	// Full page in one shot.
	result, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Sort: []search.Sort[invitation.SortFieldPlatformInvitation]{
				{Field: invitation.SortFieldPlatformInvitationCreatedAt, Direction: search.SortAscending},
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
	page1, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Sort: []search.Sort[invitation.SortFieldPlatformInvitation]{
				{Field: invitation.SortFieldPlatformInvitationCreatedAt, Direction: search.SortAscending},
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
	page2, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Sort: []search.Sort[invitation.SortFieldPlatformInvitation]{
				{Field: invitation.SortFieldPlatformInvitationCreatedAt, Direction: search.SortAscending},
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
	page3, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Sort: []search.Sort[invitation.SortFieldPlatformInvitation]{
				{Field: invitation.SortFieldPlatformInvitationCreatedAt, Direction: search.SortAscending},
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
	pageOOB, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Pagination: search.Pagination{Limit: 10, Offset: 50},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestPlatformInvitationRepositorySearchByTenantIDSortDirections(t *testing.T) {
	tenantID := createTenant(t)

	// Emails chosen so ascending/descending email order is unambiguous, and
	// created sequentially so insertion order (== CreatedAt/UpdatedAt
	// ascending order) coincides with alphabetical email order too — one
	// fixture set exercises all three sort fields unambiguously.
	emails := []string{"alpha@example.com", "bravo@example.com", "charlie@example.com"}
	expectedAscending := make([]string, 0, len(emails))
	for _, email := range emails {
		entity := invitation.PlatformInvitation{
			PlatformTenantID: tenantID,
			Email:            email,
			Code:             uniqueName(t, "code"),
		}
		entity.GenerateID()
		created, err := repoCtx.InvitationRepository.Create(context.Background(), entity)
		require.NoError(t, err)
		expectedAscending = append(expectedAscending, created.ID)
	}
	expectedDescending := []string{expectedAscending[2], expectedAscending[1], expectedAscending[0]}

	for _, field := range []invitation.SortFieldPlatformInvitation{
		invitation.SortFieldPlatformInvitationCreatedAt,
		invitation.SortFieldPlatformInvitationUpdatedAt,
		invitation.SortFieldPlatformInvitationEmail,
	} {
		t.Run(string(field), func(t *testing.T) {
			asc, err := repoCtx.InvitationRepository.SearchByTenantID(
				context.Background(), tenantID,
				search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
					Sort: []search.Sort[invitation.SortFieldPlatformInvitation]{
						{Field: field, Direction: search.SortAscending},
					},
					Pagination: search.Pagination{Limit: 10, Offset: 0},
				},
			)
			require.NoError(t, err)
			require.Len(t, asc.Items, 3)
			assert.Equal(t, expectedAscending,
				[]string{asc.Items[0].ID, asc.Items[1].ID, asc.Items[2].ID})

			desc, err := repoCtx.InvitationRepository.SearchByTenantID(
				context.Background(), tenantID,
				search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
					Sort: []search.Sort[invitation.SortFieldPlatformInvitation]{
						{Field: field, Direction: search.SortDescending},
					},
					Pagination: search.Pagination{Limit: 10, Offset: 0},
				},
			)
			require.NoError(t, err)
			require.Len(t, desc.Items, 3)
			assert.Equal(t, expectedDescending,
				[]string{desc.Items[0].ID, desc.Items[1].ID, desc.Items[2].ID})
		})
	}
}

func TestPlatformInvitationRepositorySearchByTenantIDFilterByIDs(t *testing.T) {
	tenantID := createTenant(t)

	first := createInvitation(t, tenantID)
	second := createInvitation(t, tenantID)
	_ = createInvitation(t, tenantID) // unrelated third invitation, must not match

	byID, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Filter:     &invitation.SearchPlatformInvitationFilter{IDs: []string{first.ID, second.ID}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byID.Total)
	gotIDs := []string{byID.Items[0].ID, byID.Items[1].ID}
	assert.ElementsMatch(t, []string{first.ID, second.ID}, gotIDs)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Filter:     &invitation.SearchPlatformInvitationFilter{IDs: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestPlatformInvitationRepositorySearchByTenantIDFilterByEmails(t *testing.T) {
	tenantID := createTenant(t)

	first := createInvitation(t, tenantID)
	second := createInvitation(t, tenantID)
	_ = createInvitation(t, tenantID) // unrelated third invitation, must not match

	byEmail, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Filter:     &invitation.SearchPlatformInvitationFilter{Emails: []string{first.Email, second.Email}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byEmail.Total)
	gotIDs := []string{byEmail.Items[0].ID, byEmail.Items[1].ID}
	assert.ElementsMatch(t, []string{first.ID, second.ID}, gotIDs)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Filter:     &invitation.SearchPlatformInvitationFilter{Emails: []string{"nobody@example.com"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestPlatformInvitationRepositorySearchByTenantIDFilterByCode(t *testing.T) {
	tenantID := createTenant(t)

	target := createInvitation(t, tenantID)
	_ = createInvitation(t, tenantID) // unrelated second invitation, must not match

	byCode, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Filter:     &invitation.SearchPlatformInvitationFilter{Code: &target.Code},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), byCode.Total)
	require.Len(t, byCode.Items, 1)
	assert.Equal(t, target.ID, byCode.Items[0].ID)

	// A code that matches nothing returns an empty, not an error.
	missingCode := "does-not-exist-code"
	none, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Filter:     &invitation.SearchPlatformInvitationFilter{Code: &missingCode},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestPlatformInvitationRepositorySearchByTenantIDFullTextSearch(t *testing.T) {
	tenantID := createTenant(t)

	matchingEmail := uniqueName(t, "AtlasMigration") + "@example.com"
	matching := invitation.PlatformInvitation{
		PlatformTenantID: tenantID,
		Email:            matchingEmail,
		Code:             uniqueName(t, "code"),
	}
	matching.GenerateID()
	createdMatching, err := repoCtx.InvitationRepository.Create(context.Background(), matching)
	require.NoError(t, err)

	other := invitation.PlatformInvitation{
		PlatformTenantID: tenantID,
		Email:            uniqueName(t, "BillingConsole") + "@example.com",
		Code:             uniqueName(t, "code"),
	}
	other.GenerateID()
	_, err = repoCtx.InvitationRepository.Create(context.Background(), other)
	require.NoError(t, err)

	// The filter is a plain SQL LIKE, not tsvector — it is case-sensitive, so
	// the term must match the stored case.
	term := "AtlasMigration"
	result, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			FullTextSearch: &term,
			Pagination:     search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, createdMatching.ID, result.Items[0].ID)
}

func TestPlatformInvitationRepositorySearchByTenantIDScopesToTenant(t *testing.T) {
	tenantID := createTenant(t)
	otherTenantID := createTenant(t)

	inScope := createInvitation(t, tenantID)
	_ = createInvitation(t, otherTenantID) // must never leak into the first tenant's results

	result, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	require.Len(t, result.Items, 1)
	assert.Equal(t, inScope.ID, result.Items[0].ID)

	otherResult, err := repoCtx.InvitationRepository.SearchByTenantID(
		context.Background(), otherTenantID,
		search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
}

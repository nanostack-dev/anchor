package repository_test

import (
	"context"
	"testing"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/platform"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createAuthUser inserts a row into the standalone `users` table and returns
// its ID. platform_users.user_id has a FOREIGN KEY onto users(id), and
// PlatformTenantUserRepository.Create does not create that row itself, so
// every platform user fixture needs one of these first. UserRepository isn't
// wired into repoCtx, so this goes straight through go-jet against repoCtx.DB.
func createAuthUser(t *testing.T) string {
	t.Helper()

	id := ids.MustNew("user")
	name := uniqueName(t, "Test User")
	entity := model.Users{
		ID:             id,
		Email:          uniqueName(t, "user") + "@example.com",
		Name:           &name,
		HashedPassword: "hashed-password",
		Status:         "active",
	}

	stmt := table.Users.INSERT(
		table.Users.AllColumns.Except(table.Users.CreatedAt, table.Users.UpdatedAt),
	).MODEL(entity)

	err := transactor.Exec(context.Background(), repoCtx.DB, stmt).Err()
	require.NoError(t, err)

	return id
}

// createPlatformUser inserts a platform user fixture under tenantID and
// returns the created domain value. Email is disambiguated via uniqueName
// since it is unique per tenant (see FindByTenantIDAndEmail).
func createPlatformUser(t *testing.T, tenantID string) platform.User {
	t.Helper()

	entity := platform.User{
		PlatformTenantID: tenantID,
		UserID:           createAuthUser(t),
		Name:             uniqueName(t, "Test Platform User"),
		Email:            uniqueName(t, "platform-user") + "@example.com",
		HashedPassword:   "hashed-password",
		Role:             platform.TenantRoleAdmin,
	}
	entity.GenerateID()

	created, err := repoCtx.PlatformTenantUserRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created
}

func TestPlatformTenantUserRepositorySearchByTenantIDEmpty(t *testing.T) {
	tenantID := createTenant(t)

	result, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.Equal(t, 0, result.Count)
	assert.Empty(t, result.Items)
}

func TestPlatformTenantUserRepositorySearchByTenantIDPaginationBoundaries(t *testing.T) {
	tenantID := createTenant(t)

	const total = 5
	created := make([]string, 0, total)
	for range total {
		created = append(created, createPlatformUser(t, tenantID).ID)
	}

	// Full page in one shot.
	result, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Sort: []search.Sort[platform.SortFieldPlatformUser]{
				{Field: platform.SortFieldPlatformUserCreatedAt, Direction: search.SortAscending},
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
	page1, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Sort: []search.Sort[platform.SortFieldPlatformUser]{
				{Field: platform.SortFieldPlatformUserCreatedAt, Direction: search.SortAscending},
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
	page2, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Sort: []search.Sort[platform.SortFieldPlatformUser]{
				{Field: platform.SortFieldPlatformUserCreatedAt, Direction: search.SortAscending},
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
	page3, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Sort: []search.Sort[platform.SortFieldPlatformUser]{
				{Field: platform.SortFieldPlatformUserCreatedAt, Direction: search.SortAscending},
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
	pageOOB, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Pagination: search.Pagination{Limit: 10, Offset: 50},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(total), pageOOB.Total)
	assert.Equal(t, 0, pageOOB.Count)
	assert.Empty(t, pageOOB.Items)
}

func TestPlatformTenantUserRepositorySearchByTenantIDSortDirections(t *testing.T) {
	tenantID := createTenant(t)

	type fixture struct {
		email string
		name  string
		role  platform.TenantRole
	}
	fixtures := []fixture{
		{email: "a-" + uniqueName(t, "alpha") + "@example.com", name: "Alpha User", role: platform.TenantRoleAdmin},
		{email: "b-" + uniqueName(t, "bravo") + "@example.com", name: "Bravo User", role: platform.TenantRoleOwner},
		{email: "c-" + uniqueName(t, "charlie") + "@example.com", name: "Charlie User", role: platform.TenantRoleAdmin},
	}

	ids := make([]string, len(fixtures))
	for i, f := range fixtures {
		entity := platform.User{
			PlatformTenantID: tenantID,
			UserID:           createAuthUser(t),
			Name:             f.name,
			Email:            f.email,
			HashedPassword:   "hashed-password",
			Role:             f.role,
		}
		entity.GenerateID()
		created, err := repoCtx.PlatformTenantUserRepository.Create(context.Background(), entity)
		require.NoError(t, err)
		ids[i] = created.ID
	}

	testCases := []struct {
		field platform.SortFieldPlatformUser
	}{
		{field: platform.SortFieldPlatformUserCreatedAt},
		{field: platform.SortFieldPlatformUserUpdatedAt},
		{field: platform.SortFieldPlatformUserEmail},
		{field: platform.SortFieldPlatformUserRole},
	}

	for _, tc := range testCases {
		t.Run(string(tc.field), func(t *testing.T) {
			asc, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
				context.Background(), tenantID,
				search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
					Sort: []search.Sort[platform.SortFieldPlatformUser]{
						{Field: tc.field, Direction: search.SortAscending},
					},
					Pagination: search.Pagination{Limit: 10, Offset: 0},
				},
			)
			require.NoError(t, err)
			require.Len(t, asc.Items, len(fixtures))

			desc, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
				context.Background(), tenantID,
				search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
					Sort: []search.Sort[platform.SortFieldPlatformUser]{
						{Field: tc.field, Direction: search.SortDescending},
					},
					Pagination: search.Pagination{Limit: 10, Offset: 0},
				},
			)
			require.NoError(t, err)
			require.Len(t, desc.Items, len(fixtures))

			ascIDs := make([]string, len(asc.Items))
			for i, item := range asc.Items {
				ascIDs[i] = item.ID
			}
			descIDs := make([]string, len(desc.Items))
			for i, item := range desc.Items {
				descIDs[i] = item.ID
			}

			// All fixture IDs are present regardless of direction.
			assert.ElementsMatch(t, ids, ascIDs)
			assert.ElementsMatch(t, ids, descIDs)

			if tc.field == platform.SortFieldPlatformUserRole {
				// Role only has two distinct values across the fixtures (two
				// ADMIN, one OWNER), so ties between the ADMIN rows are not
				// ordered deterministically — assert on the role sequence
				// instead of exact ID reversal.
				ascRoles := make([]platform.TenantRole, len(asc.Items))
				for i, item := range asc.Items {
					ascRoles[i] = item.Role
				}
				descRoles := make([]platform.TenantRole, len(desc.Items))
				for i, item := range desc.Items {
					descRoles[i] = item.Role
				}
				assert.Equal(t,
					[]platform.TenantRole{platform.TenantRoleAdmin, platform.TenantRoleAdmin, platform.TenantRoleOwner},
					ascRoles,
				)
				assert.Equal(t,
					[]platform.TenantRole{platform.TenantRoleOwner, platform.TenantRoleAdmin, platform.TenantRoleAdmin},
					descRoles,
				)
				return
			}

			// Every other field is unique per fixture, so descending must be
			// the exact reverse of ascending.
			reversedDesc := make([]string, len(descIDs))
			for i, id := range descIDs {
				reversedDesc[len(descIDs)-1-i] = id
			}
			assert.Equal(t, ascIDs, reversedDesc)
		})
	}

	// Email sort has an unambiguous ordering by construction: verify directly.
	ascByEmail, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Sort: []search.Sort[platform.SortFieldPlatformUser]{
				{Field: platform.SortFieldPlatformUserEmail, Direction: search.SortAscending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, ascByEmail.Items, 3)
	assert.Equal(t, []string{ids[0], ids[1], ids[2]},
		[]string{ascByEmail.Items[0].ID, ascByEmail.Items[1].ID, ascByEmail.Items[2].ID})

	descByEmail, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Sort: []search.Sort[platform.SortFieldPlatformUser]{
				{Field: platform.SortFieldPlatformUserEmail, Direction: search.SortDescending},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, descByEmail.Items, 3)
	assert.Equal(t, []string{ids[2], ids[1], ids[0]},
		[]string{descByEmail.Items[0].ID, descByEmail.Items[1].ID, descByEmail.Items[2].ID})
}

func TestPlatformTenantUserRepositorySearchByTenantIDFilterByIDs(t *testing.T) {
	tenantID := createTenant(t)

	first := createPlatformUser(t, tenantID)
	second := createPlatformUser(t, tenantID)
	_ = createPlatformUser(t, tenantID) // unrelated third user, must not match

	byID, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Filter:     &platform.SearchPlatformUserFilter{IDs: []string{first.ID, second.ID}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byID.Total)
	gotIDs := []string{byID.Items[0].ID, byID.Items[1].ID}
	assert.ElementsMatch(t, []string{first.ID, second.ID}, gotIDs)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Filter:     &platform.SearchPlatformUserFilter{IDs: []string{"does-not-exist"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestPlatformTenantUserRepositorySearchByTenantIDFilterByEmails(t *testing.T) {
	tenantID := createTenant(t)

	first := createPlatformUser(t, tenantID)
	second := createPlatformUser(t, tenantID)
	_ = createPlatformUser(t, tenantID) // unrelated third user, must not match

	byEmail, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Filter:     &platform.SearchPlatformUserFilter{Emails: []string{first.Email, second.Email}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byEmail.Total)
	gotIDs := []string{byEmail.Items[0].ID, byEmail.Items[1].ID}
	assert.ElementsMatch(t, []string{first.ID, second.ID}, gotIDs)

	// A filter matching nothing returns an empty, not an error.
	none, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Filter:     &platform.SearchPlatformUserFilter{Emails: []string{"nobody@example.com"}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestPlatformTenantUserRepositorySearchByTenantIDFilterByRoles(t *testing.T) {
	tenantID := createTenant(t)

	admin := platform.User{
		PlatformTenantID: tenantID,
		UserID:           createAuthUser(t),
		Name:             uniqueName(t, "Admin User"),
		Email:            uniqueName(t, "admin-user") + "@example.com",
		HashedPassword:   "hashed-password",
		Role:             platform.TenantRoleAdmin,
	}
	admin.GenerateID()
	createdAdmin, err := repoCtx.PlatformTenantUserRepository.Create(context.Background(), admin)
	require.NoError(t, err)

	owner := platform.User{
		PlatformTenantID: tenantID,
		UserID:           createAuthUser(t),
		Name:             uniqueName(t, "Owner User"),
		Email:            uniqueName(t, "owner-user") + "@example.com",
		HashedPassword:   "hashed-password",
		Role:             platform.TenantRoleOwner,
	}
	owner.GenerateID()
	_, err = repoCtx.PlatformTenantUserRepository.Create(context.Background(), owner)
	require.NoError(t, err)

	byRole, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Filter:     &platform.SearchPlatformUserFilter{Roles: []platform.TenantRole{platform.TenantRoleAdmin}},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, byRole.Items, 1)
	assert.Equal(t, createdAdmin.ID, byRole.Items[0].ID)

	// A role not present under this tenant returns an empty, not an error.
	none, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Filter: &platform.SearchPlatformUserFilter{
				Roles: []platform.TenantRole{"NOT_A_REAL_ROLE"},
			},
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), none.Total)
	assert.Empty(t, none.Items)
}

func TestPlatformTenantUserRepositorySearchByTenantIDFullTextSearch(t *testing.T) {
	tenantID := createTenant(t)

	entity := platform.User{
		PlatformTenantID: tenantID,
		UserID:           createAuthUser(t),
		Name:             "Atlas Migration Owner",
		Email:            uniqueName(t, "atlas-owner") + "@example.com",
		HashedPassword:   "hashed-password",
		Role:             platform.TenantRoleOwner,
	}
	entity.GenerateID()
	matching, err := repoCtx.PlatformTenantUserRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	other := platform.User{
		PlatformTenantID: tenantID,
		UserID:           createAuthUser(t),
		Name:             "Billing Console Admin",
		Email:            uniqueName(t, "billing-admin") + "@example.com",
		HashedPassword:   "hashed-password",
		Role:             platform.TenantRoleAdmin,
	}
	other.GenerateID()
	_, err = repoCtx.PlatformTenantUserRepository.Create(context.Background(), other)
	require.NoError(t, err)

	// The filter is a plain SQL LIKE, not tsvector — it is case-sensitive, so
	// the term must match the stored case.
	term := "Atlas"
	result, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			FullTextSearch: &term,
			Pagination:     search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, matching.ID, result.Items[0].ID)
}

func TestPlatformTenantUserRepositorySearchByTenantIDScopesToTenant(t *testing.T) {
	tenantID := createTenant(t)
	otherTenantID := createTenant(t)

	inScope := createPlatformUser(t, tenantID)
	_ = createPlatformUser(t, otherTenantID) // must never leak into the first tenant's results

	result, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), tenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, inScope.ID, result.Items[0].ID)

	otherResult, err := repoCtx.PlatformTenantUserRepository.SearchByTenantID(
		context.Background(), otherTenantID,
		search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]{
			Pagination: search.Pagination{Limit: 10, Offset: 0},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), otherResult.Total)
}

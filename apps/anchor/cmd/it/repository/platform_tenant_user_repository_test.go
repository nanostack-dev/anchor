package repository_test

import (
	"context"
	"testing"
	"time"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/platform"

	"github.com/nanostack-dev/nanostack-framework/db/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
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
// since it is unique per tenant (see FindByPlatformTenantIDAndEmail).
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

// createPlatformUserFor inserts a platform user for an existing auth user
// under tenantID and sets its created_at. The tests below use it to give one
// auth user several platform user rows with a known order.
func createPlatformUserFor(t *testing.T, tenantID, userID string, createdAt time.Time) platform.User {
	t.Helper()

	entity := platform.User{
		PlatformTenantID: tenantID,
		UserID:           userID,
		Name:             uniqueName(t, "Test Platform User"),
		Email:            uniqueName(t, "platform-user") + "@example.com",
		HashedPassword:   "hashed-password",
		Role:             platform.TenantRoleAdmin,
	}
	entity.GenerateID()

	created, err := repoCtx.PlatformTenantUserRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	stmt := table.PlatformUsers.UPDATE(table.PlatformUsers.CreatedAt).
		SET(postgres.TimestampzT(createdAt)).
		WHERE(table.PlatformUsers.ID.EQ(postgres.String(created.ID)))
	require.NoError(t, transactor.Exec(context.Background(), repoCtx.DB, stmt).Err())

	return created
}

// The auth middleware calls FindByPlatformTenantIDAndUserID on each platform
// bearer request. A user removed from tenant A must not resolve through the
// row of tenant B.
func TestPlatformTenantUserRepositoryFindByPlatformTenantIDAndUserIDScopesToTenant(t *testing.T) {
	ctx := context.Background()
	tenantA, tenantB := createTenant(t), createTenant(t)
	userID := createAuthUser(t)
	base := time.Now().Add(-time.Hour)
	rowB := createPlatformUserFor(t, tenantB, userID, base)
	rowA := createPlatformUserFor(t, tenantA, userID, base.Add(time.Minute))

	got, err := repoCtx.PlatformTenantUserRepository.FindByPlatformTenantIDAndUserID(ctx, tenantA, userID)
	require.NoError(t, err)
	require.True(t, got.IsPresent())
	assert.Equal(t, rowA.ID, got.Value().ID)

	// The derived finders and the derived delete do not cross tenants.
	cross, err := repoCtx.PlatformTenantUserRepository.FindByPlatformTenantIDAndID(ctx, tenantA, rowB.ID)
	require.NoError(t, err)
	assert.True(t, cross.IsAbsent())
	cross, err = repoCtx.PlatformTenantUserRepository.FindByPlatformTenantIDAndEmail(ctx, tenantA, rowB.Email)
	require.NoError(t, err)
	assert.True(t, cross.IsAbsent())
	require.NoError(t, repoCtx.PlatformTenantUserRepository.DeleteByPlatformTenantIDAndID(ctx, tenantA, rowB.ID))
	still, err := repoCtx.PlatformTenantUserRepository.FindByPlatformTenantIDAndID(ctx, tenantB, rowB.ID)
	require.NoError(t, err)
	assert.True(t, still.IsPresent())

	// Remove the user from tenant A.
	require.NoError(t, repoCtx.PlatformTenantUserRepository.DeleteByPlatformTenantIDAndID(ctx, tenantA, rowA.ID))

	got, err = repoCtx.PlatformTenantUserRepository.FindByPlatformTenantIDAndUserID(ctx, tenantA, userID)
	require.NoError(t, err)
	assert.True(t, got.IsAbsent())

	got, err = repoCtx.PlatformTenantUserRepository.FindByPlatformTenantIDAndUserID(ctx, tenantB, userID)
	require.NoError(t, err)
	require.True(t, got.IsPresent())
	assert.Equal(t, rowB.ID, got.Value().ID)
}

// idx_platform_users_tenant_user keeps one platform user per user in a tenant,
// so FindByPlatformTenantIDAndUserID never has to pick between rows.
func TestPlatformTenantUserRepositoryRefusesASecondRowForTheSameTenantAndUser(t *testing.T) {
	ctx := context.Background()
	tenantID := createTenant(t)
	userID := createAuthUser(t)
	first := createPlatformUserFor(t, tenantID, userID, time.Now().Add(-time.Hour))

	second := platform.User{
		PlatformTenantID: tenantID,
		UserID:           userID,
		Name:             uniqueName(t, "Test Platform User"),
		Email:            uniqueName(t, "platform-user") + "@example.com",
		HashedPassword:   "hashed-password",
		Role:             platform.TenantRoleAdmin,
	}
	second.GenerateID()
	_, err := repoCtx.PlatformTenantUserRepository.Create(ctx, second)
	require.ErrorContains(t, err, "idx_platform_users_tenant_user")

	got, err := repoCtx.PlatformTenantUserRepository.FindByPlatformTenantIDAndUserID(ctx, tenantID, userID)
	require.NoError(t, err)
	require.True(t, got.IsPresent())
	assert.Equal(t, first.ID, got.Value().ID)
}

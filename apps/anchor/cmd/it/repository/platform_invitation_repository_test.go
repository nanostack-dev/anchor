package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/invitation"

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

func TestPlatformInvitationRepositoryFindByPlatformTenantIDAndEmailScopesToTenant(t *testing.T) {
	ctx := context.Background()
	tenantA, tenantB := createTenant(t), createTenant(t)
	created := createInvitation(t, tenantB)

	cross, err := repoCtx.InvitationRepository.FindByPlatformTenantIDAndEmail(ctx, tenantA, created.Email)
	require.NoError(t, err)
	assert.True(t, cross.IsAbsent())

	own, err := repoCtx.InvitationRepository.FindByPlatformTenantIDAndEmail(ctx, tenantB, created.Email)
	require.NoError(t, err)
	require.True(t, own.IsPresent())
	assert.Equal(t, created.ID, own.Value().ID)
}

// FindByCodeAndEmailInternal runs before the tenant is known, so it matches on
// code and email only.
func TestPlatformInvitationRepositoryFindByCodeAndEmailInternal(t *testing.T) {
	ctx := context.Background()
	created := createInvitation(t, createTenant(t))

	got, err := repoCtx.InvitationRepository.FindByCodeAndEmailInternal(ctx, created.Code, created.Email)
	require.NoError(t, err)
	require.True(t, got.IsPresent())
	assert.Equal(t, created.ID, got.Value().ID)
	assert.Equal(t, created.PlatformTenantID, got.Value().PlatformTenantID)

	wrong, err := repoCtx.InvitationRepository.FindByCodeAndEmailInternal(ctx, created.Code, "other-"+created.Email)
	require.NoError(t, err)
	assert.True(t, wrong.IsAbsent())
}

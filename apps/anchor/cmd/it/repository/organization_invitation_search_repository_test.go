package repository_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/organizationinvitation"
	invitationrepo "anchor/internal/invitation/repository"
)

func createOrganizationInvitation(
	t *testing.T, repo invitationrepo.Repository, productID, organizationID string, expiresAt time.Time,
) organizationinvitation.Invitation {
	t.Helper()

	entity := organizationinvitation.Invitation{
		ProductID: productID, OrganizationID: organizationID,
		Email: uniqueName(t, "invitee") + "@example.com", ExpiresAt: expiresAt,
	}
	entity.GenerateID()
	created, err := repo.Create(context.Background(), entity)
	require.NoError(t, err)

	return created
}

func TestInvitationSearchFiltersByStatusOnOneClock(t *testing.T) {
	repo := invitationrepo.NewRepository(repoCtx.DB, slog.New(slog.DiscardHandler))
	productID, organizationID := tenantProductOrgChain(t)
	now := time.Now()
	pending := createOrganizationInvitation(t, repo, productID, organizationID, now.Add(time.Hour))
	expired := createOrganizationInvitation(t, repo, productID, organizationID, now.Add(-time.Hour))
	accepted := createOrganizationInvitation(t, repo, productID, organizationID, now.Add(time.Hour))
	_, err := repo.MarkAccepted(context.Background(), productID, organizationID, accepted.ID, now)
	require.NoError(t, err)

	scope := invitationrepo.OrganizationInvitationScope{ProductID: productID, OrganizationID: organizationID}
	for status, want := range map[organizationinvitation.Status]string{
		organizationinvitation.StatusPending:  pending.ID,
		organizationinvitation.StatusExpired:  expired.ID,
		organizationinvitation.StatusAccepted: accepted.ID,
	} {
		result, searchErr := repo.SearchInOrganization(context.Background(),
			invitationrepo.NewOrganizationInvitationSearch(scope), []organizationinvitation.Status{status})
		require.NoError(t, searchErr)
		require.Len(t, result.Items, 1, "status %s", status)
		assert.Equal(t, want, result.Items[0].ID)
		assert.Equal(t, status, result.Items[0].Status)
	}

	all, err := repo.SearchInProduct(context.Background(), invitationrepo.NewProductInvitationSearch(productID), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), all.Total)
}

// A row that expires while the test runs is pending on one search and expired
// on a later one, never both and never neither, and the status it returns is the
// status the filter selected it by.
func TestInvitationSearchFilterAndReturnedStatusAgreeAcrossTheExpiryBoundary(t *testing.T) {
	repo := invitationrepo.NewRepository(repoCtx.DB, slog.New(slog.DiscardHandler))
	productID, organizationID := tenantProductOrgChain(t)
	boundary := createOrganizationInvitation(t, repo, productID, organizationID, time.Now().Add(150*time.Millisecond))
	scope := invitationrepo.OrganizationInvitationScope{ProductID: productID, OrganizationID: organizationID}

	sawPending, sawExpired := false, false
	for deadline := time.Now().Add(500 * time.Millisecond); time.Now().Before(deadline); {
		for _, status := range []organizationinvitation.Status{
			organizationinvitation.StatusPending, organizationinvitation.StatusExpired,
		} {
			result, err := repo.SearchInOrganization(context.Background(),
				invitationrepo.NewOrganizationInvitationSearch(scope), []organizationinvitation.Status{status})
			require.NoError(t, err)
			for _, item := range result.Items {
				assert.Equal(t, boundary.ID, item.ID)
				assert.Equal(t, status, item.Status, "the filter and the returned status read one clock")
				sawPending = sawPending || status == organizationinvitation.StatusPending
				sawExpired = sawExpired || status == organizationinvitation.StatusExpired
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.True(t, sawPending && sawExpired, "the loop must cross the expiry boundary")
}

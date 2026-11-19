package service_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"anchor/internal/events"
)

// givenAFailingEndpoint stores an event endpoint at url for a new product and
// records failedCalls failed calls to it.
func givenAFailingEndpoint(t *testing.T, repo events.EndpointRepository, url string, failedCalls int) string {
	t.Helper()
	owner := GivenATenantAndProduct(t)
	require.NoError(t, repo.Upsert(t.Context(), events.Endpoint{
		ProductID:              owner.Product.ID,
		PlatformTenantID:       owner.Tenant.ID,
		URL:                    url,
		SigningSecretEncrypted: "v1:fixture",
		Events:                 []string{string(events.OrganizationCreated)},
		DeliveryStatus:         "never_attempted",
	}))
	for range failedCalls {
		require.NoError(t, repo.RecordDeliveryResultInternal(t.Context(), owner.Product.ID, url, false))
	}
	return owner.Product.ID
}

// The worker removes an endpoint that answers 410 Gone. The URL and the count
// are checked in the delete itself, so the guard holds in SQL.
func TestDeleteGoneEventEndpoint(t *testing.T) {
	repo := events.NewEndpointRepository(TestDB, slog.New(slog.DiscardHandler))

	t.Run("An endpoint with fewer failed calls than the threshold is kept", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/hooks", 2)

		removed, err := repo.DeleteGoneEndpointInternal(t.Context(), productID, "https://example.com/hooks", 3)

		require.NoError(t, err)
		require.False(t, removed)
		found, err := repo.FindByProductIDInternal(t.Context(), productID)
		require.NoError(t, err)
		require.True(t, found.IsPresent())
	})

	t.Run("A gone call to an old URL keeps the endpoint saved at a new URL", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/new-hooks", 5)

		removed, err := repo.DeleteGoneEndpointInternal(t.Context(), productID, "https://example.com/old-hooks", 3)

		require.NoError(t, err)
		require.False(t, removed)
		found, err := repo.FindByProductIDInternal(t.Context(), productID)
		require.NoError(t, err)
		require.True(t, found.IsPresent())
		require.Equal(t, "https://example.com/new-hooks", found.Value().URL)
	})

	t.Run("An endpoint at the URL with enough failed calls is removed", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/hooks", 3)

		removed, err := repo.DeleteGoneEndpointInternal(t.Context(), productID, "https://example.com/hooks", 3)

		require.NoError(t, err)
		require.True(t, removed)
		found, err := repo.FindByProductIDInternal(t.Context(), productID)
		require.NoError(t, err)
		require.True(t, found.IsAbsent())
	})
}

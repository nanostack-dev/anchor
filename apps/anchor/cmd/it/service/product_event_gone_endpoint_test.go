package service_test

import (
	"log/slog"
	"testing"
	"time"

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

// The worker removes an endpoint that answers 410 Gone. The URL, the count and
// the age of the failure streak are checked in the delete itself, so the guard
// holds in SQL.
func TestDeleteGoneEventEndpoint(t *testing.T) {
	repo := events.NewEndpointRepository(TestDB, slog.New(slog.DiscardHandler))

	t.Run("An endpoint with fewer failed calls than the threshold is kept", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/hooks", 2)

		removed, err := repo.DeleteGoneEndpointInternal(t.Context(), productID, "https://example.com/hooks", 3, 0)

		require.NoError(t, err)
		require.False(t, removed)
		found, err := repo.FindByProductIDInternal(t.Context(), productID)
		require.NoError(t, err)
		require.True(t, found.IsPresent())
	})

	t.Run("A gone call to an old URL keeps the endpoint saved at a new URL", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/new-hooks", 5)

		removed, err := repo.DeleteGoneEndpointInternal(t.Context(), productID, "https://example.com/old-hooks", 3, 0)

		require.NoError(t, err)
		require.False(t, removed)
		found, err := repo.FindByProductIDInternal(t.Context(), productID)
		require.NoError(t, err)
		require.True(t, found.IsPresent())
		require.Equal(t, "https://example.com/new-hooks", found.Value().URL)
	})

	t.Run("An endpoint at the URL with enough failed calls is removed", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/hooks", 3)

		removed, err := repo.DeleteGoneEndpointInternal(t.Context(), productID, "https://example.com/hooks", 3, 0)

		require.NoError(t, err)
		require.True(t, removed)
		found, err := repo.FindByProductIDInternal(t.Context(), productID)
		require.NoError(t, err)
		require.True(t, found.IsAbsent())
	})

	t.Run("A streak younger than the window keeps the endpoint", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/hooks", 3)

		removed, err := repo.DeleteGoneEndpointInternal(
			t.Context(),
			productID,
			"https://example.com/hooks",
			3,
			time.Hour,
		)

		require.NoError(t, err)
		require.False(t, removed)
	})

	t.Run("A streak older than the window is removed", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/hooks", 3)
		_, err := TestDB.ExecContext(t.Context(),
			`UPDATE product_event_endpoint_configs SET failing_since = NOW() - INTERVAL '2 hours'
			 WHERE product_id = $1`, productID)
		require.NoError(t, err)

		removed, err := repo.DeleteGoneEndpointInternal(
			t.Context(),
			productID,
			"https://example.com/hooks",
			3,
			time.Hour,
		)

		require.NoError(t, err)
		require.True(t, removed)
	})

	t.Run("An endpoint whose streak has no start is kept", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/hooks", 5)
		_, err := TestDB.ExecContext(t.Context(),
			`UPDATE product_event_endpoint_configs SET failing_since = NULL WHERE product_id = $1`, productID)
		require.NoError(t, err)

		removed, err := repo.DeleteGoneEndpointInternal(t.Context(), productID, "https://example.com/hooks", 3, 0)

		require.NoError(t, err)
		require.False(t, removed)
	})
}

// A failed call dates the streak; a success or a save of the endpoint clears it.
func TestEventEndpointFailureStreak(t *testing.T) {
	repo := events.NewEndpointRepository(TestDB, slog.New(slog.DiscardHandler))
	stored := func(t *testing.T, productID string) events.Endpoint {
		t.Helper()
		found, err := repo.FindByProductIDInternal(t.Context(), productID)
		require.NoError(t, err)
		require.True(t, found.IsPresent())
		return found.Value()
	}

	t.Run("The first failed call starts the streak and later ones keep its start", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/hooks", 1)
		first := stored(t, productID)
		require.False(t, first.FailingSince.IsZero())
		require.False(t, first.LastFailedAt.IsZero())

		require.NoError(
			t,
			repo.RecordDeliveryResultInternal(t.Context(), productID, "https://example.com/hooks", false),
		)

		second := stored(t, productID)
		require.True(t, first.FailingSince.Equal(second.FailingSince))
		require.False(t, second.LastFailedAt.Before(first.LastFailedAt))
	})

	t.Run("A success clears the streak", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/hooks", 2)

		require.NoError(t, repo.RecordDeliveryResultInternal(t.Context(), productID, "https://example.com/hooks", true))

		endpoint := stored(t, productID)
		require.True(t, endpoint.FailingSince.IsZero())
		require.True(t, endpoint.LastFailedAt.IsZero())
	})

	t.Run("Saving the endpoint again clears the streak", func(t *testing.T) {
		productID := givenAFailingEndpoint(t, repo, "https://example.com/hooks", 2)
		endpoint := stored(t, productID)
		endpoint.ConsecutiveFailedCalls = 0
		endpoint.DeliveryStatus = "never_attempted"
		endpoint.FailingSince = time.Time{}
		endpoint.LastFailedAt = time.Time{}

		require.NoError(t, repo.Upsert(t.Context(), endpoint))

		saved := stored(t, productID)
		require.True(t, saved.FailingSince.IsZero())
		require.True(t, saved.LastFailedAt.IsZero())
	})
}

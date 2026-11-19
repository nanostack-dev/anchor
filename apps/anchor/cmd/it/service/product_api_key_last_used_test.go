package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/product/apikey"
)

// The validation path reads the key through a 15-minute cache whose
// LastUsedAt is never refreshed, so the hourly throttle has to hold in SQL.
func TestProductAPIKeyLastUsedAtIsWrittenAtMostHourly(t *testing.T) {
	readKey := func(t *testing.T, productID, id string) apikey.ProductAPIKey {
		t.Helper()
		found, err := APIKeyRepository.GetByProductAndID(t.Context(), productID, id)
		require.NoError(t, err)
		key, ok := found.Get()
		require.True(t, ok)
		return key
	}

	t.Run("repeated validations write the row once", func(t *testing.T) {
		tenantAndProduct := GivenATenantAndProduct(t)
		permissions := GivenBasicAnchorPermissions(t, tenantAndProduct.Product.ID)
		key, value := GivenAPIKey(t, tenantAndProduct.Product.ID, permissions)
		input := apikey.ValidateAPIKeyScopesInput{
			ProductID: tenantAndProduct.Product.ID, APIKeyValue: value, Scopes: permissions,
		}

		_, err := APIKeyService.ValidateAPIKeyAndScopes(t.Context(), input)
		require.NoError(t, err)
		first := readKey(t, tenantAndProduct.Product.ID, key.ID)
		require.NotNil(t, first.LastUsedAt)

		_, err = APIKeyService.ValidateAPIKeyAndScopes(t.Context(), input)
		require.NoError(t, err)
		second := readKey(t, tenantAndProduct.Product.ID, key.ID)

		require.NotNil(t, second.LastUsedAt)
		assert.True(t, first.LastUsedAt.Equal(*second.LastUsedAt),
			"last_used_at moved from %s to %s", first.LastUsedAt, second.LastUsedAt)
		assert.True(t, first.UpdatedAt.Equal(second.UpdatedAt),
			"updated_at moved from %s to %s", first.UpdatedAt, second.UpdatedAt)
	})

	t.Run("an inactive key is never written", func(t *testing.T) {
		tenantAndProduct := GivenATenantAndProduct(t)
		permissions := GivenBasicAnchorPermissions(t, tenantAndProduct.Product.ID)
		key, value := GivenADeactivatedAPIKey(t, tenantAndProduct.Product.ID, permissions)

		_, err := APIKeyService.ValidateAPIKeyAndScopes(t.Context(), apikey.ValidateAPIKeyScopesInput{
			ProductID: tenantAndProduct.Product.ID, APIKeyValue: value, Scopes: permissions,
		})
		require.Error(t, err)

		assert.Nil(t, readKey(t, tenantAndProduct.Product.ID, key.ID).LastUsedAt)
	})

	t.Run("a use older than an hour is advanced", func(t *testing.T) {
		tenantAndProduct := GivenATenantAndProduct(t)
		permissions := GivenBasicAnchorPermissions(t, tenantAndProduct.Product.ID)
		key, _ := GivenAPIKey(t, tenantAndProduct.Product.ID, permissions)
		_, err := TestDB.ExecContext(t.Context(),
			`UPDATE product_api_keys SET last_used_at = now() - interval '2 hours' WHERE id = $1`, key.ID)
		require.NoError(t, err)
		before := readKey(t, tenantAndProduct.Product.ID, key.ID)

		require.NoError(t, APIKeyRepository.UpdateLastUsedAt(t.Context(), tenantAndProduct.Product.ID, key.ID))

		after := readKey(t, tenantAndProduct.Product.ID, key.ID)
		require.NotNil(t, after.LastUsedAt)
		assert.WithinDuration(t, time.Now(), *after.LastUsedAt, time.Minute)
		assert.True(t, after.LastUsedAt.After(*before.LastUsedAt))
	})

	t.Run("a use within the hour is left alone", func(t *testing.T) {
		tenantAndProduct := GivenATenantAndProduct(t)
		permissions := GivenBasicAnchorPermissions(t, tenantAndProduct.Product.ID)
		key, _ := GivenAPIKey(t, tenantAndProduct.Product.ID, permissions)
		require.NoError(t, APIKeyRepository.UpdateLastUsedAt(t.Context(), tenantAndProduct.Product.ID, key.ID))
		before := readKey(t, tenantAndProduct.Product.ID, key.ID)

		require.NoError(t, APIKeyRepository.UpdateLastUsedAt(t.Context(), tenantAndProduct.Product.ID, key.ID))

		after := readKey(t, tenantAndProduct.Product.ID, key.ID)
		require.NotNil(t, after.LastUsedAt)
		assert.True(t, before.LastUsedAt.Equal(*after.LastUsedAt))
	})
}

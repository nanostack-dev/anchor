package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/product/apikey"

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

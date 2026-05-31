package dslfactory

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	"anchor/internal/domain/product"
	"anchor/internal/service"
)

type ProductResult struct {
	ID string
}

func CreateProductWithDefaultPermissions(
	t require.TestingT,
	tenantID string,
) *ProductResult {
	require.NotNil(t, itshared.ProductRepository, "product repository is not available in test setup")
	require.NotNil(
		t,
		itshared.PermissionRepository,
		"permission repository is not available in test setup",
	)

	prod := product.Product{
		PlatformTenantID: tenantID,
		Name:             "Test Product " + itshared.Faker.UUID().V4(),
		Description:      "Test product for integration tests",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	prod.GenerateID()

	createdProduct, err := itshared.ProductRepository.Create(context.Background(), prod)
	require.NoError(t, err)

	for _, perm := range service.GeneratePermissions() {
		perm.ProductID = createdProduct.ID
		_, permErr := itshared.PermissionRepository.Create(context.Background(), perm)
		require.NoError(t, permErr)
	}

	return &ProductResult{ID: createdProduct.ID}
}

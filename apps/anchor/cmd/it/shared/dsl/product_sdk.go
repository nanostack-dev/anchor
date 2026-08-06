package itdsl

import (
	"context"

	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	"anchor/internal/domain/permission"
	"anchor/internal/domain/product/apikey"
	"anchor/internal/service"
)

// SDKClient mints an all-scope product API key and returns an anchorsdk client
// bound to it and to this product.
//
// It exists so CT can drive the SDK the way a product backend does — over HTTP,
// against the running server — which is what keeps the SDK honest about the
// contract anchor actually serves. Prefer [ProductContext.AllScopeAPIKeyClient]
// when a test needs to assert raw status codes or typed error bodies; the SDK
// deliberately hides both.
func (tp *ProductContext) SDKClient() *anchorsdk.Client {
	tp.testingContext.Helper()

	require.NotNil(
		tp.testingContext,
		itshared.APIKeyService,
		"api key service is not available in test setup",
	)

	allScopes := slicex.Map(
		service.GeneratePermissions(),
		func(productPermission permission.ProductPermission) string {
			return productPermission.Name
		},
	)

	_, clearAPIKey, err := itshared.APIKeyService.Create(
		context.Background(),
		apikey.CreateProductAPIKeyInput{
			ProductID:   tp.ProductID,
			Name:        "Test SDK API Key " + itshared.Faker.UUID().V4(),
			Description: new("Product API key for anchorsdk integration tests"),
			Permissions: allScopes,
		},
	)
	require.NoError(tp.testingContext, err)

	sdk, err := anchorsdk.New(anchorsdk.Config{
		BaseURL:       tp.ServerURL,
		ProductID:     tp.ProductID,
		ProductAPIKey: clearAPIKey,
	})
	require.NoError(tp.testingContext, err)

	return sdk
}

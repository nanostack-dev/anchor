package itdsl

import (
	"context"

	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	"anchor/internal/domain/permission"
	"anchor/internal/domain/product/apikey"
	"anchor/internal/service"
)

// SDKClient mints a product API key and returns an anchorsdk client bound to it
// and to this product. Naming no scope mints a key with every scope; name
// scopes only when the authorization is the subject of the test.
//
// It exists so CT can drive the SDK the way a product backend does — over HTTP,
// against the running server — which is what keeps the SDK honest about the
// contract anchor actually serves. Prefer [ProductContext.AllScopeAPIKeyClient]
// when a test needs to assert raw status codes or typed error bodies; the SDK
// deliberately hides both.
func (tp *ProductContext) SDKClient(scopes ...string) *anchorsdk.Client {
	tp.testingContext.Helper()

	require.NotNil(
		tp.testingContext,
		itshared.APIKeyService,
		"api key service is not available in test setup",
	)

	if len(scopes) == 0 {
		scopes = functional.Slice(service.GeneratePermissions()).Map(
			func(productPermission permission.ProductPermission) string { return productPermission.Name },
		)
	}
	_, clearAPIKey, err := itshared.APIKeyService.Create(
		context.Background(),
		apikey.CreateProductAPIKeyInput{
			ProductID:   tp.ProductID,
			Name:        "Test SDK API Key " + itshared.Faker.UUID().V4(),
			Description: new("Product API key for anchorsdk integration tests"),
			Permissions: scopes,
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

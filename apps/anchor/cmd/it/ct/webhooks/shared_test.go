package webhooks_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/internal/repository"
)

// SecretRepo is populated by fx.Populate so tests can assert on rotation state
// that the API deliberately never exposes.
var SecretRepo repository.WebhookEndpointSecretRepository

func TestMain(m *testing.M) {
	// The shared setup resolves application.yaml relative to the working
	// directory, which lives one level up for this sub-package.
	if err := os.Chdir(".."); err != nil {
		panic(err)
	}

	itshared.RunTestMain(
		m, itshared.TestConfig{
			EnableRedis:            true,
			PopulateRepositories:   true,
			APIKeyService:          &itshared.APIKeyService,
			PermissionRepository:   &itshared.PermissionRepository,
			ProductRepository:      &itshared.ProductRepository,
			ProductUserRepository:  &itshared.ProductUserRepository,
			TenantRepository:       &itshared.TenantRepository,
			UserRepository:         &itshared.UserRepository,
			PlatformUserRepository: &itshared.PlatformTenantUserRepo,
			JWTHelper:              &itshared.JWTHelper,
			ExtraPopulateTargets:   []any{&SecretRepo},
		},
	)
}

func createTestProductContext(t *testing.T) *itdsl.ProductContext {
	t.Helper()

	tenantAlias := "tenant.webhooks." + itshared.Faker.UUID().V4()
	productAlias := "product.webhooks." + itshared.Faker.UUID().V4()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: tenantAlias}).
		Product(itdsl.ProductOpts{Alias: productAlias, TenantAlias: tenantAlias}).
		Build()

	return state.Product(productAlias)
}

// unauthenticatedClient talks to the API with no credentials at all.
func unauthenticatedClient(t *testing.T) *ct.ClientWithResponses {
	t.Helper()

	client, err := ct.NewClientWithResponses(itshared.ServerURL)
	require.NoError(t, err)

	return client
}

// createEndpoint registers a webhook endpoint and returns the create response,
// which is the only place besides rotation where the secret is visible.
func createEndpoint(
	t *testing.T,
	productContext *itdsl.ProductContext,
	url string,
	eventTypes []string,
) ct.WebhookEndpointWithSecretResponse {
	t.Helper()

	resp, err := productContext.OwnerAuthenticatedClient().CreateWebhookEndpointWithResponse(
		context.Background(),
		productContext.ProductID,
		ct.CreateWebhookEndpointJSONRequestBody{
			Url:         url,
			Description: new("ct webhook endpoint"),
			EventTypes:  eventTypes,
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode())
	require.NotNil(t, resp.JSON201)

	return *resp.JSON201
}

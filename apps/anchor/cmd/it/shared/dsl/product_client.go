package itdsl

import (
	"context"
	"net/http"
	"testing"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	"anchor/internal/domain/permission"
	"anchor/internal/domain/product/apikey"
	"anchor/internal/service"
)

func getDefaultProductResourcePermissionsStrings() []string {
	return []string{"file:read", "file:create", "file:update", "file:delete"}
}

func (tp *ProductContext) OwnerAuthenticatedClient() *nanostackClient.ClientWithResponses {
	return tp.ownerAuthenticatedClient
}

func (tp *ProductContext) AllScopeAPIKeyClient() *nanostackClient.ClientWithResponses {
	return tp.allScopeAPIKeyClient
}

func (tp *ProductContext) CreateProductResourcePermissions(
	testingCtx *testing.T,
	names ...string,
) []nanostackClient.ProductResourcePermissionResponse {
	var permissions []nanostackClient.ProductResourcePermissionResponse
	for _, name := range names {
		resp, err := tp.OwnerAuthenticatedClient().CreateProductResourcePermissionWithResponse(
			context.Background(),
			tp.ProductID,
			nanostackClient.CreateProductResourcePermissionJSONRequestBody{
				Name:          name,
				Description:   new("Test resource permission for integration tests"),
				ScopeModifier: new("GLOBAL"),
			},
		)
		require.NoError(testingCtx, err)
		require.Equal(testingCtx, http.StatusCreated, resp.StatusCode())
		require.NotNil(testingCtx, resp.JSON201)
		permissions = append(permissions, *resp.JSON201)
	}
	return permissions
}

func (tp *ProductContext) CreateDefaultProductResourcePermissions(
	testingCtx *testing.T,
) []nanostackClient.ProductResourcePermissionResponse {
	tp.DefaultResourcePermissions = tp.CreateProductResourcePermissions(
		testingCtx,
		getDefaultProductResourcePermissionsStrings()...,
	)
	return tp.DefaultResourcePermissions
}

func (tp *ProductContext) CreateOrganization(
	testingCtx *testing.T,
	name string,
	description *string,
) nanostackClient.ProductOrganizationResponse {
	resp, err := tp.AllScopeAPIKeyClient().CreateProductOrganizationWithResponse(
		context.Background(),
		tp.ProductID,
		nanostackClient.CreateProductOrganizationJSONRequestBody{
			Name:        name,
			Description: description,
		},
	)
	require.NoError(testingCtx, err)
	require.Equal(testingCtx, http.StatusCreated, resp.StatusCode())
	require.NotNil(testingCtx, resp.JSON201)

	return *resp.JSON201
}

func (tp *ProductContext) CreateAPIKeyClientWithAllScopes() (
	*nanostackClient.ClientWithResponses,
	string,
) {
	allScopes := functional.Slice(
		service.GeneratePermissions()).Map(

		func(permission permission.ProductPermission) string {
			return permission.Name
		})

	return tp.createAPIKeyClientWithScopes(allScopes)
}

func (tp *ProductContext) CreateAPIKeyClientWithScopes(scopes []string) (
	*nanostackClient.ClientWithResponses,
	string,
) {
	return tp.createAPIKeyClientWithScopes(scopes)
}

func (tp *ProductContext) createAPIKeyClientWithScopes(scopes []string) (
	*nanostackClient.ClientWithResponses,
	string,
) {
	require.NotNil(
		tp.testingContext,
		itshared.APIKeyService,
		"api key service is not available in test setup",
	)

	createdKey, clearAPIKey, err := itshared.APIKeyService.Create(
		context.Background(),
		apikey.CreateProductAPIKeyInput{
			ProductID:   tp.ProductID,
			Name:        "Test API Key " + itshared.Faker.UUID().V4(),
			Description: new("Test API key for integration tests"),
			Permissions: scopes,
		},
	)
	require.NoError(tp.testingContext, err)

	client, err := nanostackClient.NewClientWithResponses(
		tp.ServerURL,
		nanostackClient.WithRequestEditorFn(
			func(_ context.Context, req *http.Request) error {
				req.Header.Add("X-Product-Api-Key", clearAPIKey)
				return nil
			},
		),
	)
	require.NoError(tp.testingContext, err)

	return client, createdKey.ID
}

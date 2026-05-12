package service_test

import (
	"strings"
	"testing"

	"anchor/internal/security"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/nanostack-dev/shared/toolkit"

	"anchor/internal/domain/permission"
	"anchor/internal/domain/product/apikey"
	"anchor/internal/service"
)

func TestApiKeyValidation(t *testing.T) {
	availableScopes := toolkit.TransformSlice(
		service.GeneratePermissions(), func(t permission.ProductPermission) string {
			return t.Name
		},
	)
	t.Run(
		"Valid API Key", func(t *testing.T) {
			tenantAndProduct := GivenATenantAndProduct(t)
			permissions := GivenBasicAnchorPermissions(
				t, tenantAndProduct.Product.ID,
			)
			apiKey, value := GivenAPIKey(
				t, tenantAndProduct.Product.ID, permissions,
			)
			validatedAPIKey, err := APIKeyService.ValidateAPIKeyAndScopes(
				t.Context(), apikey.ValidateAPIKeyScopesInput{
					ProductID:   tenantAndProduct.Product.ID,
					APIKeyValue: value,
					Scopes:      permissions,
				},
			)
			require.NoError(t, err, "Failed to validate API key scopes")
			assert.Equal(t, validatedAPIKey.ProductID, tenantAndProduct.Product.ID)
			assert.Equal(t, validatedAPIKey.ID, apiKey.ID)
		},
	)
	t.Run(
		"Legacy Prefix API Key", func(t *testing.T) {
			tenantAndProduct := GivenATenantAndProduct(t)
			permissions := GivenBasicAnchorPermissions(
				t, tenantAndProduct.Product.ID,
			)
			anchorValue, err := security.GenerateProductAPIKey()
			require.NoError(t, err)

			legacyValue := strings.Replace(
				anchorValue,
				security.DefaultProductAPIKeyPrefix,
				"nanostack_prd_apikey_",
				1,
			)
			apiKey := apikey.ProductAPIKey{
				ProductID:       tenantAndProduct.Product.ID,
				Name:            Faker.RandomStringWithLength(20),
				Mutable:         true,
				HashedValue:     security.HashSecret(legacyValue),
				ObfuscatedValue: "nanostack_prd_apikey_***_legacy",
				Status:          apikey.StatusActive,
			}
			apiKey.GenerateID()
			apiKey.Permissions = toolkit.TransformSlice(
				permissions,
				func(perm string) apikey.ProductAPIKeyPermission {
					return apikey.ProductAPIKeyPermission{
						APIKeyID:       apiKey.ID,
						ProductID:      tenantAndProduct.Product.ID,
						PermissionName: perm,
					}
				},
			)
			createdAPIKey, createErr := APIKeyRepository.Create(t.Context(), apiKey, nil)
			require.NoError(t, createErr)

			validatedAPIKey, err := APIKeyService.ValidateAPIKeyAndScopes(
				t.Context(), apikey.ValidateAPIKeyScopesInput{
					ProductID:   tenantAndProduct.Product.ID,
					APIKeyValue: legacyValue,
					Scopes:      permissions,
				},
			)
			require.NoError(t, err, "Failed to validate legacy-prefixed API key scopes")
			assert.Equal(t, tenantAndProduct.Product.ID, validatedAPIKey.ProductID)
			assert.Equal(t, createdAPIKey.ID, validatedAPIKey.ID)
		},
	)
	t.Run(
		"Invalid API Key", func(t *testing.T) {
			tenantAndProduct := GivenATenantAndProduct(t)
			GivenBasicAnchorPermissions(
				t, tenantAndProduct.Product.ID,
			)
			invalidValue := "invalid-api-key-value"
			_, err := APIKeyService.ValidateAPIKeyAndScopes(
				t.Context(), apikey.ValidateAPIKeyScopesInput{
					ProductID:   tenantAndProduct.Product.ID,
					APIKeyValue: invalidValue,
					Scopes:      []string{"file:read"},
				},
			)
			require.Error(t, err, "Expected error for invalid API key")
			assert.Contains(t, err.Error(), "Product API key is invalid")
		},
	)
	t.Run(
		"Validate api key with a scopes that doesn't exists", func(t *testing.T) {
			tenantAndProduct := GivenATenantAndProduct(t)
			GivenBasicAnchorPermissions(
				t, tenantAndProduct.Product.ID,
			)
			apiKeyCreated, value := GivenAPIKey(
				t, tenantAndProduct.Product.ID, availableScopes,
			)
			_, err := APIKeyService.ValidateAPIKeyAndScopes(
				t.Context(), apikey.ValidateAPIKeyScopesInput{
					ProductID:   tenantAndProduct.Product.ID,
					APIKeyValue: value,
					Scopes:      append(availableScopes, "file:extra"),
				},
			)
			require.Error(t, err, "Expected error for API key with extra scopes")
			assert.Contains(t, err.Error(), "Product API key does not have sufficient permissions")
			var anchorErr *toolkit.NanostackError
			require.ErrorAs(t, err, &anchorErr)
			assert.Equal(
				t, anchorErr, service.NewProductAPIKeyInsufficientPermissionsError(
					apiKeyCreated.ID, []string{"file:extra"}, availableScopes,
				),
			)
		},
	)
	t.Run(
		"Validate api key with a scopes the key doesn't have", func(t *testing.T) {
			tenantAndProduct := GivenATenantAndProduct(t)
			GivenBasicAnchorPermissions(
				t, tenantAndProduct.Product.ID,
			)
			apiKeyCreated, value := GivenAPIKey(
				t, tenantAndProduct.Product.ID, availableScopes[0:1],
			)
			_, err := APIKeyService.ValidateAPIKeyAndScopes(
				t.Context(), apikey.ValidateAPIKeyScopesInput{
					ProductID:   tenantAndProduct.Product.ID,
					APIKeyValue: value,
					Scopes:      availableScopes[1:2],
				},
			)
			require.Error(t, err, "Expected error for API key with extra scopes")
			var anchorErr *toolkit.NanostackError
			require.ErrorAs(t, err, &anchorErr)
			assert.Equal(
				t, anchorErr, service.NewProductAPIKeyInsufficientPermissionsError(
					apiKeyCreated.ID, availableScopes[1:2], availableScopes[0:1],
				),
			)
		},
	)
	t.Run(
		"Empty API Key Value", func(t *testing.T) {
			tenantAndProduct := GivenATenantAndProduct(t)
			GivenBasicAnchorPermissions(
				t, tenantAndProduct.Product.ID,
			)
			_, err := APIKeyService.ValidateAPIKeyAndScopes(
				t.Context(), apikey.ValidateAPIKeyScopesInput{
					ProductID:   tenantAndProduct.Product.ID,
					APIKeyValue: "",
					Scopes:      []string{"file:read"},
				},
			)
			require.Error(t, err, "Expected error for empty API key")
			assert.Contains(t, err.Error(), "Product API key is invalid")
		},
	)
	t.Run(
		"Wrong Product Name", func(t *testing.T) {
			tenantAndProduct := GivenATenantAndProduct(t)
			permissions := GivenBasicAnchorPermissions(
				t, tenantAndProduct.Product.ID,
			)
			_, value := GivenAPIKey(
				t, tenantAndProduct.Product.ID, permissions,
			)
			differentProduct := GivenARandomProduct(t, tenantAndProduct.Tenant.ID)
			_, err := APIKeyService.ValidateAPIKeyAndScopes(
				t.Context(), apikey.ValidateAPIKeyScopesInput{
					ProductID:   differentProduct.ID,
					APIKeyValue: value,
					Scopes:      permissions,
				},
			)
			require.Error(t, err, "Expected error for wrong product Name")
			assert.Contains(t, err.Error(), "Product API key is invalid")
		},
	)
	t.Run(
		"Deactivated API Key", func(t *testing.T) {
			tenantAndProduct := GivenATenantAndProduct(t)
			permissions := GivenBasicAnchorPermissions(
				t, tenantAndProduct.Product.ID,
			)
			_, value := GivenADeactivatedAPIKey(
				t, tenantAndProduct.Product.ID, permissions,
			)
			_, err := APIKeyService.ValidateAPIKeyAndScopes(
				t.Context(), apikey.ValidateAPIKeyScopesInput{
					ProductID:   tenantAndProduct.Product.ID,
					APIKeyValue: value,
					Scopes:      permissions,
				},
			)
			require.Error(t, err, "Expected error for deactivated API key")
			assert.Contains(t, err.Error(), "Product API key is inactive")
		},
	)
	t.Run(
		"Empty Scopes Array", func(t *testing.T) {
			tenantAndProduct := GivenATenantAndProduct(t)
			permissions := GivenBasicAnchorPermissions(
				t, tenantAndProduct.Product.ID,
			)
			_, value := GivenAPIKey(
				t, tenantAndProduct.Product.ID, permissions,
			)
			_, err := APIKeyService.ValidateAPIKeyAndScopes(
				t.Context(), apikey.ValidateAPIKeyScopesInput{
					ProductID:   tenantAndProduct.Product.ID,
					APIKeyValue: value,
					Scopes:      []string{},
				},
			)
			require.Error(t, err, "Expected error for empty scopes")
		},
	)
	t.Run(
		"Case Sensitive Scope Names", func(t *testing.T) {
			tenantAndProduct := GivenATenantAndProduct(t)
			permissions := GivenBasicAnchorPermissions(
				t, tenantAndProduct.Product.ID,
			)
			_, value := GivenAPIKey(
				t, tenantAndProduct.Product.ID, permissions,
			)
			uppercaseScopes := toolkit.TransformSlice(permissions, strings.ToUpper)
			_, err := APIKeyService.ValidateAPIKeyAndScopes(
				t.Context(), apikey.ValidateAPIKeyScopesInput{
					ProductID:   tenantAndProduct.Product.ID,
					APIKeyValue: value,
					Scopes:      uppercaseScopes,
				},
			)
			require.Error(t, err, "Expected error for case mismatch in scopes")
			assert.Contains(t, err.Error(), "Product API key does not have sufficient permissions")
		},
	)
}

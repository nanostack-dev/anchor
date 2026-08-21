package service_test

import (
	"strings"
	"testing"
	"time"

	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/organization"
	orgapikey "anchor/internal/domain/organization/apikey"
)

func TestOrganizationAPIKeyValidation(t *testing.T) {
	t.Run("Valid API Key", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)
		createdKey, value := givenOrganizationAPIKey(
			t,
			ctxData,
			[]string{ctxData.PermissionSet.FileRead, ctxData.PermissionSet.FileCreate},
			orgapikey.StatusActive,
			nil,
		)

		result, err := OrgAPIKeyService.ValidateAPIKeyAndScopes(
			t.Context(),
			orgapikey.ValidateOrganizationAPIKeyScopesInput{
				ProductID:      ctxData.Product.Product.ID,
				OrganizationID: ctxData.Organization.ID,
				APIKeyValue:    value,
				Scopes:         []string{ctxData.PermissionSet.FileRead, ctxData.PermissionSet.FileCreate},
			},
		)
		require.NoError(t, err)
		assert.True(t, result.Authorized)
		assert.False(t, result.Inactive)
		assert.Equal(t, createdKey.ID, result.APIKey.ID)
		assert.ElementsMatch(
			t,
			[]string{ctxData.PermissionSet.FileRead, ctxData.PermissionSet.FileCreate},
			result.Permissions,
		)
		assert.Empty(t, result.MissingPrivileges)

		reloaded, err := OrgAPIKeyRepository.GetByID(
			t.Context(),
			ctxData.Organization.ID,
			createdKey.ID,
		)
		require.NoError(t, err)
		require.True(t, reloaded.IsPresent())
		assert.NotNil(t, reloaded.Value().LastUsedAt)
	})

	t.Run("Missing Privileges", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)
		createdKey, value := givenOrganizationAPIKey(
			t,
			ctxData,
			[]string{ctxData.PermissionSet.FileRead},
			orgapikey.StatusActive,
			nil,
		)

		result, err := OrgAPIKeyService.ValidateAPIKeyAndScopes(
			t.Context(),
			orgapikey.ValidateOrganizationAPIKeyScopesInput{
				ProductID:      ctxData.Product.Product.ID,
				OrganizationID: ctxData.Organization.ID,
				APIKeyValue:    value,
				Scopes:         []string{ctxData.PermissionSet.FileCreate},
			},
		)
		require.NoError(t, err)
		assert.False(t, result.Authorized)
		assert.False(t, result.Inactive)
		assert.Equal(t, createdKey.ID, result.APIKey.ID)
		assert.ElementsMatch(t, []string{ctxData.PermissionSet.FileRead}, result.Permissions)
		assert.Equal(t, []string{ctxData.PermissionSet.FileCreate}, result.MissingPrivileges)
	})

	t.Run("Inactive API Key", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)
		createdKey, value := givenOrganizationAPIKey(
			t,
			ctxData,
			[]string{ctxData.PermissionSet.FileRead, ctxData.PermissionSet.FileCreate},
			orgapikey.StatusInactive,
			nil,
		)

		result, err := OrgAPIKeyService.ValidateAPIKeyAndScopes(
			t.Context(),
			orgapikey.ValidateOrganizationAPIKeyScopesInput{
				ProductID:      ctxData.Product.Product.ID,
				OrganizationID: ctxData.Organization.ID,
				APIKeyValue:    value,
				Scopes:         []string{ctxData.PermissionSet.FileRead},
			},
		)
		require.NoError(t, err)
		assert.False(t, result.Authorized)
		assert.True(t, result.Inactive)
		assert.Equal(t, createdKey.ID, result.APIKey.ID)
		assert.Empty(t, result.MissingPrivileges)
	})

	t.Run("Expired API Key", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)
		createdKey, value := givenOrganizationAPIKey(
			t,
			ctxData,
			[]string{ctxData.PermissionSet.FileRead, ctxData.PermissionSet.FileCreate},
			orgapikey.StatusActive,
			nil,
		)

		expiresAt := time.Now().Add(-time.Minute)
		createdKey.ExpiresAt = &expiresAt
		updatedKey, updateErr := OrgAPIKeyRepository.Update(t.Context(), createdKey)
		require.NoError(t, updateErr)

		result, err := OrgAPIKeyService.ValidateAPIKeyAndScopes(
			t.Context(),
			orgapikey.ValidateOrganizationAPIKeyScopesInput{
				ProductID:      ctxData.Product.Product.ID,
				OrganizationID: ctxData.Organization.ID,
				APIKeyValue:    value,
				Scopes:         []string{ctxData.PermissionSet.FileRead},
			},
		)
		require.NoError(t, err)
		assert.False(t, result.Authorized)
		assert.True(t, result.Inactive)
		assert.Equal(t, updatedKey.ID, result.APIKey.ID)
		assert.Equal(t, orgapikey.StatusInactive, result.APIKey.Status)
		assert.Empty(t, result.MissingPrivileges)
		assert.Nil(t, result.APIKey.LastUsedAt)

		reloaded, err := OrgAPIKeyRepository.GetByID(
			t.Context(),
			ctxData.Organization.ID,
			updatedKey.ID,
		)
		require.NoError(t, err)
		require.True(t, reloaded.IsPresent())
		assert.Equal(t, orgapikey.StatusInactive, reloaded.Value().Status)
	})

	t.Run("Near Future Expiration Uses Second Precision Boundary", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)
		// Truncate to the second to exercise the second-precision boundary, but
		// keep a comfortable future margin so the key cannot expire during the
		// create + validate round-trip (a 1s margin truncated down can leave
		// well under a second and flakes under CI load).
		expiresAt := time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)

		createdKey, value := givenOrganizationAPIKey(
			t,
			ctxData,
			[]string{ctxData.PermissionSet.FileRead},
			orgapikey.StatusActive,
			&expiresAt,
		)

		assert.WithinDuration(t, expiresAt, *createdKey.ExpiresAt, time.Second)

		result, err := OrgAPIKeyService.ValidateAPIKeyAndScopes(
			t.Context(),
			orgapikey.ValidateOrganizationAPIKeyScopesInput{
				ProductID:      ctxData.Product.Product.ID,
				OrganizationID: ctxData.Organization.ID,
				APIKeyValue:    value,
				Scopes:         []string{ctxData.PermissionSet.FileRead},
			},
		)
		require.NoError(t, err)
		assert.True(t, result.Authorized)
		assert.False(t, result.Inactive)
		assert.Equal(t, createdKey.ID, result.APIKey.ID)
	})

	t.Run("Legacy Prefix API Key", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)
		permissions := []string{ctxData.PermissionSet.FileRead, ctxData.PermissionSet.FileCreate}
		anchorValue, err := security.GenerateOrganizationAPIKey(security.DefaultOrganizationAPIKeyRootPrefix)
		require.NoError(t, err)

		legacyValue := strings.Replace(
			anchorValue,
			security.DefaultOrganizationAPIKeyPrefix,
			"nanostack_org_apikey_",
			1,
		)
		createdKey := orgapikey.OrganizationAPIKey{
			OrganizationID:  ctxData.Organization.ID,
			Name:            "Org Key " + Faker.UUID().V4(),
			Description:     new("Legacy organization API key for tests"),
			HashedValue:     security.HashSecret(legacyValue),
			ObfuscatedValue: "nanostack_org_apikey_***_legacy",
			Status:          orgapikey.StatusActive,
		}
		createdKey.GenerateID()
		createdKey.Permissions = functional.Slice(
			permissions).Map(

			func(perm string) orgapikey.OrganizationAPIKeyPermission {
				return orgapikey.OrganizationAPIKeyPermission{
					APIKeyID:       createdKey.ID,
					OrganizationID: ctxData.Organization.ID,
					ProductID:      ctxData.Product.Product.ID,
					PermissionName: perm,
				}
			})

		persistedKey, createErr := OrgAPIKeyRepository.Create(t.Context(), createdKey)
		require.NoError(t, createErr)

		result, err := OrgAPIKeyService.ValidateAPIKeyAndScopes(
			t.Context(),
			orgapikey.ValidateOrganizationAPIKeyScopesInput{
				ProductID:      ctxData.Product.Product.ID,
				OrganizationID: ctxData.Organization.ID,
				APIKeyValue:    legacyValue,
				Scopes:         []string{ctxData.PermissionSet.FileRead, ctxData.PermissionSet.FileCreate},
			},
		)
		require.NoError(t, err)
		assert.True(t, result.Authorized)
		assert.Equal(t, persistedKey.ID, result.APIKey.ID)
	})

	t.Run("Invalid API Key", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)

		_, err := OrgAPIKeyService.ValidateAPIKeyAndScopes(
			t.Context(),
			orgapikey.ValidateOrganizationAPIKeyScopesInput{
				ProductID:      ctxData.Product.Product.ID,
				OrganizationID: ctxData.Organization.ID,
				APIKeyValue:    "invalid-api-key-value",
				Scopes:         []string{ctxData.PermissionSet.FileRead},
			},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "The product API key is invalid.")
	})

	t.Run("Organization Does Not Belong To Product", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)
		_, value := givenOrganizationAPIKey(
			t,
			ctxData,
			[]string{ctxData.PermissionSet.FileRead},
			orgapikey.StatusActive,
			nil,
		)

		otherTenantAndProduct := GivenATenantAndProduct(t)

		_, err := OrgAPIKeyService.ValidateAPIKeyAndScopes(
			t.Context(),
			orgapikey.ValidateOrganizationAPIKeyScopesInput{
				ProductID:      otherTenantAndProduct.Product.ID,
				OrganizationID: ctxData.Organization.ID,
				APIKeyValue:    value,
				Scopes:         []string{ctxData.PermissionSet.FileRead},
			},
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, fault.ErrNotFound)
	})

	t.Run("Last Used At Is Not Updated Within One Hour", func(t *testing.T) {
		ctxData := givenOrganizationAPIKeyContext(t)
		createdKey, value := givenOrganizationAPIKey(
			t,
			ctxData,
			[]string{ctxData.PermissionSet.FileRead},
			orgapikey.StatusActive,
			nil,
		)

		firstResult, err := OrgAPIKeyService.ValidateAPIKeyAndScopes(
			t.Context(),
			orgapikey.ValidateOrganizationAPIKeyScopesInput{
				ProductID:      ctxData.Product.Product.ID,
				OrganizationID: ctxData.Organization.ID,
				APIKeyValue:    value,
				Scopes:         []string{ctxData.PermissionSet.FileRead},
			},
		)
		require.NoError(t, err)
		require.NotNil(t, firstResult.APIKey.LastUsedAt)

		firstReloadOpt, err := OrgAPIKeyRepository.GetByID(
			t.Context(),
			ctxData.Organization.ID,
			createdKey.ID,
		)
		require.NoError(t, err)
		require.True(t, firstReloadOpt.IsPresent())
		firstReload := firstReloadOpt.ToPtr()
		require.NotNil(t, firstReload.LastUsedAt)

		secondResult, secondErr := OrgAPIKeyService.ValidateAPIKeyAndScopes(
			t.Context(),
			orgapikey.ValidateOrganizationAPIKeyScopesInput{
				ProductID:      ctxData.Product.Product.ID,
				OrganizationID: ctxData.Organization.ID,
				APIKeyValue:    value,
				Scopes:         []string{ctxData.PermissionSet.FileRead},
			},
		)
		require.NoError(t, secondErr)
		require.NotNil(t, secondResult.APIKey.LastUsedAt)

		secondReloadOpt, err := OrgAPIKeyRepository.GetByID(
			t.Context(),
			ctxData.Organization.ID,
			createdKey.ID,
		)
		require.NoError(t, err)
		require.True(t, secondReloadOpt.IsPresent())
		secondReload := secondReloadOpt.ToPtr()
		require.NotNil(t, secondReload.LastUsedAt)

		assert.WithinDuration(t, *firstReload.LastUsedAt, *secondReload.LastUsedAt, time.Second)
	})
}

type organizationAPIKeyContextData struct {
	Product       TenantAndProduct
	Organization  organization.Organization
	PermissionSet organizationAPIKeyResourcePermissions
}

func givenOrganizationAPIKeyContext(t *testing.T) organizationAPIKeyContextData {
	t.Helper()

	tenantAndProduct := GivenATenantAndProduct(t)
	GivenBasicProductResourcePermissions(t, tenantAndProduct.Product.ID)

	org := organization.Organization{
		ProductID:   tenantAndProduct.Product.ID,
		Name:        Faker.Company().Name(),
		Description: new("Organization API key validation test organization"),
	}
	org.GenerateID()

	createdOrg, err := OrganizationRepo.Create(t.Context(), org)
	require.NoError(t, err)

	return organizationAPIKeyContextData{
		Product:       tenantAndProduct,
		Organization:  createdOrg,
		PermissionSet: GivenOrganizationAPIKeyResourcePermissionSet(),
	}
}

func givenOrganizationAPIKey(
	t *testing.T,
	ctxData organizationAPIKeyContextData,
	permissions []string,
	status orgapikey.Status,
	expiresAt *time.Time,
) (orgapikey.OrganizationAPIKey, string) {
	t.Helper()

	createdKey, clearValue, err := OrgAPIKeyService.Create(
		t.Context(),
		orgapikey.CreateOrganizationAPIKeyInput{
			ProductID:      ctxData.Product.Product.ID,
			OrganizationID: ctxData.Organization.ID,
			Name:           "Org Key " + Faker.UUID().V4(),
			Description:    new("Organization API key for tests"),
			ExpiresAt:      expiresAt,
			Permissions:    permissions,
		},
	)
	require.NoError(t, err)

	if status == orgapikey.StatusInactive {
		createdKey.Status = orgapikey.StatusInactive
		updated, updateErr := OrgAPIKeyRepository.Update(t.Context(), createdKey)
		require.NoError(t, updateErr)
		createdKey = updated
	}

	return createdKey, clearValue
}

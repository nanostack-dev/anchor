package repository_test

import (
	"context"
	"testing"

	orgapikey "anchor/internal/domain/organization/apikey"

	"github.com/stretchr/testify/require"
)

// createOrganizationAPIKey inserts an organization API key fixture under
// organizationID and returns its ID.
func createOrganizationAPIKey(t *testing.T, organizationID string) string {
	t.Helper()

	entity := orgapikey.OrganizationAPIKey{
		OrganizationID:  organizationID,
		Name:            uniqueName(t, "Test API Key"),
		HashedValue:     uniqueName(t, "hashed"),
		ObfuscatedValue: uniqueName(t, "obfuscated"),
		Status:          orgapikey.StatusActive,
	}
	entity.GenerateID()

	created, err := repoCtx.OrganizationAPIKeyRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	return created.ID
}

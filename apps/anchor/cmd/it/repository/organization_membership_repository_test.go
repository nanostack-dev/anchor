package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// createOrgMembership creates a role, a product user, and an organization
// membership linking them, returning the product user's ID.
func createOrgMembership(t *testing.T, productID, organizationID string) string {
	t.Helper()

	roleID := createProductRole(t, productID)
	productUserID := createProductUser(t, productID)

	_, err := repoCtx.OrganizationMembershipRepository.Create(
		context.Background(), productID, organizationID, productUserID, &roleID,
	)
	require.NoError(t, err)

	return productUserID
}

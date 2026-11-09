package repository_test

import (
	"context"
	"testing"

	"anchor/internal/domain/product/user"

	"github.com/stretchr/testify/require"
)

// createOrgMembership creates a role, a product user, and an organization
// membership linking them, returning both IDs for assertions.
//
//nolint:nonamedreturns // named per the required helper signature; both IDs are otherwise identical strings.
func createOrgMembership(t *testing.T, productID, organizationID string) (productUserID, roleID string) {
	t.Helper()

	roleID = createProductRole(t, productID)
	productUserID = createProductUser(t, productID)

	_, err := repoCtx.OrganizationMembershipRepository.Create(
		context.Background(), productID, organizationID, productUserID, &roleID,
	)
	require.NoError(t, err)

	return productUserID, roleID
}

// createOrgMembershipWithUser creates a role and a membership for an
// explicitly-constructed product user, returning the product user's ID.
func createOrgMembershipWithUser(
	t *testing.T, productID, organizationID string, entity user.ProductUser,
) string {
	t.Helper()

	entity.ProductID = productID
	if entity.Status == "" {
		entity.Status = user.ProductUserStatusActive
	}
	entity.GenerateID()

	created, err := repoCtx.ProductUserRepository.Create(context.Background(), entity)
	require.NoError(t, err)

	roleID := createProductRole(t, productID)

	_, err = repoCtx.OrganizationMembershipRepository.Create(
		context.Background(), productID, organizationID, created.ID, &roleID,
	)
	require.NoError(t, err)

	return created.ID
}

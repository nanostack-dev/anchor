package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrganizationMembershipCrossProductIsolation is a security regression test
// for a cross-tenant (cross-product) write on the organization membership
// endpoints.
//
// Anchor's tenant boundary is the product: a product API key authenticates a
// single product, and every membership write is scoped by the product id in the
// path (validated by the auth middleware). AddMember/UpdateMemberRole validated
// the product user and the role against that product, but never validated that
// the *organization* in the path belongs to it. Because the membership row and
// its FK targets (a caller-product user + caller-product role) all satisfy the
// schema, and the follow-up membership lookups scope by the user's product
// rather than the organization's, an INSERT into another product's organization
// committed and returned 201. This let a caller authenticated for product A
// plant membership rows into an organization owned by product B.
func TestOrganizationMembershipCrossProductIsolation(t *testing.T) {
	ctx := context.Background()

	// Attacker: fully authenticated for their own product.
	attacker := createTestProductContext(t)
	attackerClient, _ := attacker.CreateAPIKeyClientWithAllScopes()
	attackerUser := createDSLProductUser(t, attacker)
	attackerRole := createDSLProductRole(t, attacker, "Attacker Role", nil)

	// Victim: a different product owning an organization.
	victim := createTestProductContext(t)
	victimOrg := victim.CreateOrganization(t, "Victim Org", nil)

	t.Run("AddMemberIntoAnotherProductsOrganizationIsRejected", func(t *testing.T) {
		resp, err := attackerClient.AddOrganizationMemberWithResponse(
			ctx,
			attacker.ProductID, // caller's own product (authorized by the key)
			victimOrg.Id,       // organization owned by a different product
			ct.AddOrganizationMemberJSONRequestBody{
				ProductUserId: attackerUser.ID,
				RoleId:        attackerRole.ID,
			},
		)
		require.NoError(t, err)
		assert.Equalf(t, http.StatusNotFound, resp.StatusCode(),
			"caller must not add a member to an organization owned by another product, got %s", resp.Status())

		var errResp ct.ApiErrorResponse
		require.NoError(t, json.Unmarshal(resp.Body, &errResp))
		require.NotEmpty(t, errResp.Errors)
		assert.Equal(t, "ORGANIZATION_NOT_FOUND", errResp.Errors[0].Code)
	})

	t.Run("UpdateMemberRoleInAnotherProductsOrganizationIsRejected", func(t *testing.T) {
		resp, err := attackerClient.UpdateOrganizationMemberRoleWithResponse(
			ctx,
			attacker.ProductID,
			victimOrg.Id,
			attackerUser.ID,
			ct.UpdateOrganizationMemberRoleJSONRequestBody{
				RoleId: attackerRole.ID,
			},
		)
		require.NoError(t, err)
		assert.Equalf(t, http.StatusNotFound, resp.StatusCode(),
			"caller must not update a membership in an organization owned by another product, got %s", resp.Status())
	})

	t.Run("VictimOrganizationHasNoInjectedMembers", func(t *testing.T) {
		searchResp, err := victim.AllScopeAPIKeyClient().SearchOrganizationMembersWithResponse(
			ctx,
			victim.ProductID,
			victimOrg.Id,
			ct.SearchOrganizationMembersJSONRequestBody{},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, searchResp.StatusCode())
		require.NotNil(t, searchResp.JSON200)
		assert.Empty(t, searchResp.JSON200.Items,
			"victim organization must contain no attacker-injected members")
	})
}

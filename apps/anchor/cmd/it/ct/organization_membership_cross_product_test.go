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
// for a cross-product write on the add-member endpoint.
//
// AddMember validated that the product user and role belong to the caller's
// product, but never that the target organization does. The membership repo
// Create/FindByOrgIDAndUserID queries scope only by organization_id +
// product_user_id (they never join organizations on product_id), so a caller
// authorized for product A could pass an organization id belonging to product B
// and have a membership row written against that foreign organization. The
// write must instead be rejected with ORGANIZATION_NOT_FOUND.
func TestOrganizationMembershipCrossProductIsolation(t *testing.T) {
	ctx := context.Background()

	// Attacker: a product with a full-scope API key, its own role and user.
	attacker := createTestProductContext(t)
	attackerClient, _ := attacker.CreateAPIKeyClientWithAllScopes()
	attackerRole := createDSLProductRole(t, attacker, "Attacker Role", nil)
	attackerUser := createDSLProductUser(t, attacker)

	// Victim: a *different* product with its own organization.
	victim := createTestProductContext(t)
	victimOrg := victim.CreateOrganization(t, "Victim Org", nil)

	t.Run("AddMemberIntoAnotherProductsOrganizationIsRejected", func(t *testing.T) {
		resp, err := attackerClient.AddOrganizationMemberWithResponse(
			ctx,
			attacker.ProductID, // path product_id the API key is authorized for
			victimOrg.Id,       // organization id owned by a different product
			ct.AddOrganizationMemberJSONRequestBody{
				ProductUserId: attackerUser.ID,
				RoleId:        attackerRole.ID,
			},
		)
		require.NoError(t, err)
		require.Equalf(t, http.StatusNotFound, resp.StatusCode(),
			"attacker must not add a member into another product's organization, got %s", resp.Status())

		var errResp ct.ApiErrorResponse
		require.NoError(t, json.Unmarshal(resp.Body, &errResp))
		require.NotEmpty(t, errResp.Errors)
		assert.Equal(t, "ORGANIZATION_NOT_FOUND", errResp.Errors[0].Code)
	})

	t.Run("VictimOrganizationHasNoInjectedMember", func(t *testing.T) {
		victimClient, _ := victim.CreateAPIKeyClientWithAllScopes()
		searchResp, err := victimClient.SearchOrganizationMembersWithResponse(
			ctx,
			victim.ProductID,
			victimOrg.Id,
			ct.SearchOrganizationMembersJSONRequestBody{},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, searchResp.StatusCode())
		require.NotNil(t, searchResp.JSON200)
		assert.Empty(t, searchResp.JSON200.Items,
			"victim organization must contain no attacker-injected memberships")
	})

	t.Run("AddMemberIntoOwnOrganizationStillWorks", func(t *testing.T) {
		// Positive control: the same call against the caller's own organization
		// succeeds, so the scope check does not break the legitimate path.
		ownOrg := attacker.CreateOrganization(t, "Attacker Org", nil)
		resp, err := attackerClient.AddOrganizationMemberWithResponse(
			ctx,
			attacker.ProductID,
			ownOrg.Id,
			ct.AddOrganizationMemberJSONRequestBody{
				ProductUserId: attackerUser.ID,
				RoleId:        attackerRole.ID,
			},
		)
		require.NoError(t, err)
		require.Equalf(t, http.StatusCreated, resp.StatusCode(),
			"caller must be able to add a member to their own organization, got %s", resp.Status())
		require.NotNil(t, resp.JSON201)
		assert.Equal(t, attackerUser.ID, resp.JSON201.ProductUserId)
	})
}

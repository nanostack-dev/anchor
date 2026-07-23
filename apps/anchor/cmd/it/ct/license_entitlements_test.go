package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getEntitlements(
	t *testing.T,
	apiKeyClient *ct.ClientWithResponses,
	productID string,
	organizationID string,
) *ct.GetOrganizationEntitlementsResponse {
	t.Helper()

	resp, err := apiKeyClient.GetOrganizationEntitlementsWithResponse(
		context.Background(), productID, organizationID,
	)
	require.NoError(t, err, "get entitlements request should not error")
	return resp
}

func TestOrganizationEntitlements(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	ownerClient := testProduct.OwnerAuthenticatedClient()
	apiKeyClient, _ := testProduct.CreateAPIKeyClientWithScopes([]string{"license:read"})

	planEntitlements := ct.EntitlementsMap{
		"api_access":      boolEntitlement(true),
		"beta_access":     boolEntitlement(false),
		"limits.max_runs": numericEntitlement(10),
	}
	pro := createPlan(t, ownerClient, testProduct.ProductID, ct.CreatePlanJSONRequestBody{
		Key: "pro", Name: "Pro", Entitlements: &planEntitlements,
	})

	t.Run("ActiveLicenseResolvesOverridesOverPlan", func(t *testing.T) {
		org := testProduct.CreateOrganization(t, "Entitlements Org", nil)
		overrides := ct.EntitlementsMap{
			"limits.max_runs": numericEntitlement(100),
		}
		putResp, err := ownerClient.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{
				PlanId:                 pro.Id,
				EntitlementOverrides:   &overrides,
				RefreshIntervalSeconds: new(3600),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, putResp.StatusCode())

		resp := getEntitlements(t, apiKeyClient, testProduct.ProductID, org.Id)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)

		assert.Equal(t, org.Id, resp.JSON200.OrganizationId)
		assert.Equal(t, testProduct.ProductID, resp.JSON200.ProductId)
		assert.Equal(t, "pro", resp.JSON200.PlanKey)
		assert.Equal(t, ct.EffectiveLicenseStatusACTIVE, resp.JSON200.Status)
		assert.Nil(t, resp.JSON200.ExpiresAt)
		assert.Nil(t, resp.JSON200.GraceUntil)

		// refresh_after sits one refresh interval out.
		assert.WithinDuration(
			t, time.Now().Add(time.Hour), resp.JSON200.RefreshAfter, 30*time.Second,
		)

		// Override wins over the plan default; plan-only values pass through.
		assert.Equal(
			t, numericEntitlement(100), resp.JSON200.Entitlements["limits.max_runs"],
			"per-organization override must win over the plan value",
		)
		assert.Equal(t, boolEntitlement(true), resp.JSON200.Entitlements["api_access"])
		assert.Equal(t, boolEntitlement(false), resp.JSON200.Entitlements["beta_access"])
	})

	t.Run("DefaultPlanFallbackForOrgWithoutLicense", func(t *testing.T) {
		fallbackProduct := createTestProductContext(t)
		fallbackOwner := fallbackProduct.OwnerAuthenticatedClient()
		fallbackAPIKeyClient, _ := fallbackProduct.CreateAPIKeyClientWithScopes(
			[]string{"license:read"},
		)
		freeEntitlements := ct.EntitlementsMap{
			"api_access":      boolEntitlement(true),
			"limits.max_runs": numericEntitlement(3),
		}
		createPlan(t, fallbackOwner, fallbackProduct.ProductID, ct.CreatePlanJSONRequestBody{
			Key: "free", Name: "Free", Entitlements: &freeEntitlements, IsDefault: new(true),
		})
		org := fallbackProduct.CreateOrganization(t, "Fallback Org", nil)

		resp := getEntitlements(
			t, fallbackAPIKeyClient, fallbackProduct.ProductID, org.Id,
		)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)

		assert.Equal(t, "free", resp.JSON200.PlanKey)
		assert.Equal(t, ct.EffectiveLicenseStatusACTIVE, resp.JSON200.Status)
		assert.Nil(t, resp.JSON200.GraceUntil)
		assert.Equal(
			t, numericEntitlement(3), resp.JSON200.Entitlements["limits.max_runs"],
		)
	})

	t.Run("NoLicenseAndNoDefaultPlanReturns404", func(t *testing.T) {
		bareProduct := createTestProductContext(t)
		bareAPIKeyClient, _ := bareProduct.CreateAPIKeyClientWithScopes([]string{"license:read"})
		org := bareProduct.CreateOrganization(t, "Unlicensed Org", nil)

		resp := getEntitlements(t, bareAPIKeyClient, bareProduct.ProductID, org.Id)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})

	t.Run("SuspendedLicenseResolvesWithSuspendedStatus", func(t *testing.T) {
		org := testProduct.CreateOrganization(t, "Suspended Org", nil)
		putResp, err := ownerClient.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{PlanId: pro.Id},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, putResp.StatusCode())
		suspendResp, err := ownerClient.SuspendOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, suspendResp.StatusCode())

		resp := getEntitlements(t, apiKeyClient, testProduct.ProductID, org.Id)
		require.Equal(
			t, http.StatusOK, resp.StatusCode(),
			"suspended licenses still resolve so consumers can render the block",
		)
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, ct.EffectiveLicenseStatusSUSPENDED, resp.JSON200.Status)
		assert.Equal(t, "pro", resp.JSON200.PlanKey)
	})

	t.Run("ExpiredWithinGraceResolvesWithGraceStatus", func(t *testing.T) {
		org := testProduct.CreateOrganization(t, "Grace Org", nil)
		expiresAt := time.Now().Add(-1 * time.Hour).UTC()
		graceUntil := time.Now().Add(72 * time.Hour).UTC()
		putResp, err := ownerClient.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{
				PlanId: pro.Id, ExpiresAt: &expiresAt, GraceUntil: &graceUntil,
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, putResp.StatusCode())

		resp := getEntitlements(t, apiKeyClient, testProduct.ProductID, org.Id)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)

		assert.Equal(t, ct.EffectiveLicenseStatusGRACE, resp.JSON200.Status)
		require.NotNil(t, resp.JSON200.ExpiresAt)
		assert.WithinDuration(t, expiresAt, *resp.JSON200.ExpiresAt, time.Second)
		require.NotNil(t, resp.JSON200.GraceUntil)
		assert.WithinDuration(t, graceUntil, *resp.JSON200.GraceUntil, time.Second)
	})

	t.Run("RevokedLicenseReturns409", func(t *testing.T) {
		org := testProduct.CreateOrganization(t, "Revoked Org", nil)
		putResp, err := ownerClient.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{PlanId: pro.Id},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, putResp.StatusCode())
		revokeResp, err := ownerClient.RevokeOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, revokeResp.StatusCode())

		resp := getEntitlements(t, apiKeyClient, testProduct.ProductID, org.Id)
		require.Equal(t, http.StatusConflict, resp.StatusCode())
		require.NotNil(t, resp.JSON409)
		assert.Equal(t, "LICENSE_REVOKED", resp.JSON409.Errors[0].Code)
	})

	t.Run("PastGraceReturns409Expired", func(t *testing.T) {
		org := testProduct.CreateOrganization(t, "Expired Org", nil)
		expiresAt := time.Now().Add(-48 * time.Hour).UTC()
		graceUntil := time.Now().Add(-24 * time.Hour).UTC()
		putResp, err := ownerClient.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{
				PlanId: pro.Id, ExpiresAt: &expiresAt, GraceUntil: &graceUntil,
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, putResp.StatusCode())

		resp := getEntitlements(t, apiKeyClient, testProduct.ProductID, org.Id)
		require.Equal(t, http.StatusConflict, resp.StatusCode())
		require.NotNil(t, resp.JSON409)
		assert.Equal(t, "LICENSE_EXPIRED", resp.JSON409.Errors[0].Code)
	})

	t.Run("ExpiredWithoutGraceWindowReturns409Expired", func(t *testing.T) {
		org := testProduct.CreateOrganization(t, "Expired No Grace Org", nil)
		expiresAt := time.Now().Add(-1 * time.Hour).UTC()
		putResp, err := ownerClient.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{PlanId: pro.Id, ExpiresAt: &expiresAt},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, putResp.StatusCode())

		resp := getEntitlements(t, apiKeyClient, testProduct.ProductID, org.Id)
		require.Equal(t, http.StatusConflict, resp.StatusCode())
		require.NotNil(t, resp.JSON409)
		assert.Equal(t, "LICENSE_EXPIRED", resp.JSON409.Errors[0].Code)
	})

	t.Run("OrganizationOfAnotherProductReturns404", func(t *testing.T) {
		otherProduct := createTestProductContext(t)
		otherOrg := otherProduct.CreateOrganization(t, "Foreign Org", nil)

		resp := getEntitlements(t, apiKeyClient, testProduct.ProductID, otherOrg.Id)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

func TestOrganizationEntitlementsAuthz(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	ownerClient := testProduct.OwnerAuthenticatedClient()
	plan := createPlan(t, ownerClient, testProduct.ProductID, ct.CreatePlanJSONRequestBody{
		Key: "pro", Name: "Pro",
	})
	org := testProduct.CreateOrganization(t, "Authz Org", nil)
	putResp, err := ownerClient.PutOrganizationLicenseWithResponse(
		ctx, testProduct.ProductID, org.Id,
		ct.PutOrganizationLicenseJSONRequestBody{PlanId: plan.Id},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, putResp.StatusCode())

	t.Run("MissingLicenseScopeReturns403", func(t *testing.T) {
		limitedClient, limitedKeyID := testProduct.CreateAPIKeyClientWithScopes(
			[]string{"organization:read"},
		)

		resp, reqErr := limitedClient.GetOrganizationEntitlementsWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, reqErr)
		itshared.AssertProductAPIKeyInsufficientPermissions(
			t, resp, limitedKeyID, []string{"license:read"}, []string{"organization:read"},
		)
	})

	t.Run("OtherProductsAPIKeyIsRejected", func(t *testing.T) {
		otherProduct := createTestProductContext(t)
		otherProductClient, _ := otherProduct.CreateAPIKeyClientWithScopes(
			[]string{"license:read"},
		)

		resp, reqErr := otherProductClient.GetOrganizationEntitlementsWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, reqErr)
		assert.Contains(
			t, []int{http.StatusUnauthorized, http.StatusForbidden}, resp.StatusCode(),
			"an API key of another product must not read entitlements for this product",
		)
	})

	t.Run("UnauthenticatedRequestReturns401", func(t *testing.T) {
		unauthenticatedClient, clientErr := ct.NewClientWithResponses(itshared.ServerURL)
		require.NoError(t, clientErr)

		resp, reqErr := unauthenticatedClient.GetOrganizationEntitlementsWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, reqErr)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
	})
}

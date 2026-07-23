package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseLifecycle(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	client := testProduct.OwnerAuthenticatedClient()
	planEntitlements := ct.EntitlementsMap{
		"api_access":      boolEntitlement(true),
		"limits.max_runs": numericEntitlement(10),
	}
	pro := createPlan(t, client, testProduct.ProductID, ct.CreatePlanJSONRequestBody{
		Key: "pro", Name: "Pro", Entitlements: &planEntitlements,
	})
	org := testProduct.CreateOrganization(t, "Lifecycle Org", nil)

	expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Second)
	graceUntil := expiresAt.Add(7 * 24 * time.Hour)

	t.Run("AssignLicense", func(t *testing.T) {
		overrides := ct.EntitlementsMap{
			"limits.max_runs": numericEntitlement(100),
		}
		resp, err := client.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{
				PlanId:                 pro.Id,
				ExpiresAt:              &expiresAt,
				GraceUntil:             &graceUntil,
				EntitlementOverrides:   &overrides,
				RefreshIntervalSeconds: new(3600),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)

		assert.NotEmpty(t, resp.JSON200.Id)
		assert.Equal(t, testProduct.ProductID, resp.JSON200.ProductId)
		assert.Equal(t, org.Id, resp.JSON200.OrganizationId)
		assert.Equal(t, pro.Id, resp.JSON200.PlanId)
		assert.Equal(t, ct.LicenseStatusACTIVE, resp.JSON200.Status)
		require.NotNil(t, resp.JSON200.ExpiresAt)
		assert.True(t, resp.JSON200.ExpiresAt.Equal(expiresAt))
		require.NotNil(t, resp.JSON200.GraceUntil)
		assert.True(t, resp.JSON200.GraceUntil.Equal(graceUntil))
		assert.Equal(t, 3600, resp.JSON200.RefreshIntervalSeconds)
		require.Len(t, resp.JSON200.EntitlementOverrides, 1)
		assert.Equal(
			t, numericEntitlement(100), resp.JSON200.EntitlementOverrides["limits.max_runs"],
		)
	})

	t.Run("GetLicense", func(t *testing.T) {
		resp, err := client.GetOrganizationLicenseWithResponse(ctx, testProduct.ProductID, org.Id)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, pro.Id, resp.JSON200.PlanId)
		assert.Equal(
			t, numericEntitlement(100), resp.JSON200.EntitlementOverrides["limits.max_runs"],
		)
	})

	t.Run("SuspendReinstateRevokeTransitions", func(t *testing.T) {
		suspendResp, err := client.SuspendOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, suspendResp.StatusCode())
		require.NotNil(t, suspendResp.JSON200)
		assert.Equal(t, ct.LicenseStatusSUSPENDED, suspendResp.JSON200.Status)

		reinstateResp, err := client.ReinstateOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, reinstateResp.StatusCode())
		require.NotNil(t, reinstateResp.JSON200)
		assert.Equal(t, ct.LicenseStatusACTIVE, reinstateResp.JSON200.Status)

		revokeResp, err := client.RevokeOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, revokeResp.StatusCode())
		require.NotNil(t, revokeResp.JSON200)
		assert.Equal(t, ct.LicenseStatusREVOKED, revokeResp.JSON200.Status)

		// Reinstate works from REVOKED too.
		reinstateAgain, err := client.ReinstateOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, err)
		require.NotNil(t, reinstateAgain.JSON200)
		assert.Equal(t, ct.LicenseStatusACTIVE, reinstateAgain.JSON200.Status)
	})

	t.Run("PutIsFullReplacement", func(t *testing.T) {
		// A second PUT without expiry/grace/overrides/interval clears them all.
		resp, err := client.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{PlanId: pro.Id},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)

		assert.Nil(t, resp.JSON200.ExpiresAt, "PUT must clear omitted expires_at")
		assert.Nil(t, resp.JSON200.GraceUntil, "PUT must clear omitted grace_until")
		assert.Empty(t, resp.JSON200.EntitlementOverrides, "PUT must clear omitted overrides")
		assert.Equal(
			t, 86400, resp.JSON200.RefreshIntervalSeconds,
			"PUT resets the refresh interval to default",
		)
		assert.Equal(
			t, ct.LicenseStatusACTIVE, resp.JSON200.Status,
			"PUT without status keeps the current status",
		)
	})
}

func TestLicenseList(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	client := testProduct.OwnerAuthenticatedClient()
	plan := createPlan(t, client, testProduct.ProductID, ct.CreatePlanJSONRequestBody{
		Key: "pro", Name: "Pro",
	})

	firstOrg := testProduct.CreateOrganization(t, "List Org One", nil)
	secondOrg := testProduct.CreateOrganization(t, "List Org Two", nil)

	for _, orgID := range []string{firstOrg.Id, secondOrg.Id} {
		resp, err := client.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, orgID,
			ct.PutOrganizationLicenseJSONRequestBody{PlanId: plan.Id},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
	}

	resp, err := client.ListLicensesWithResponse(ctx, testProduct.ProductID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.NotNil(t, resp.JSON200)
	require.Len(t, resp.JSON200.Items, 2, "list is scoped to the product")

	orgIDs := []string{resp.JSON200.Items[0].OrganizationId, resp.JSON200.Items[1].OrganizationId}
	assert.ElementsMatch(t, []string{firstOrg.Id, secondOrg.Id}, orgIDs)
}

func TestLicenseAssignmentNegatives(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	client := testProduct.OwnerAuthenticatedClient()
	plan := createPlan(t, client, testProduct.ProductID, ct.CreatePlanJSONRequestBody{
		Key: "pro", Name: "Pro",
	})
	org := testProduct.CreateOrganization(t, "Negative Org", nil)

	t.Run("GetWithoutLicenseReturns404", func(t *testing.T) {
		resp, err := client.GetOrganizationLicenseWithResponse(ctx, testProduct.ProductID, org.Id)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})

	t.Run("PutWithUnknownPlanReturns400", func(t *testing.T) {
		resp, err := client.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{PlanId: "plan_2ikcVW44U7UtqJHCOTqHuwkgrBb"},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode())
		require.NotNil(t, resp.JSON400)
		assert.Equal(t, "PLAN_REFERENCE_INVALID", resp.JSON400.Errors[0].Code)
	})

	t.Run("PutWithGraceBeforeExpiryReturns400", func(t *testing.T) {
		expiresAt := time.Now().Add(48 * time.Hour)
		graceUntil := expiresAt.Add(-24 * time.Hour)
		resp, err := client.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{
				PlanId: plan.Id, ExpiresAt: &expiresAt, GraceUntil: &graceUntil,
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode())
		require.NotNil(t, resp.JSON400)
		assert.Equal(t, "INVALID_LICENSE_GRACE", resp.JSON400.Errors[0].Code)
	})

	t.Run("PutWithTooSmallRefreshIntervalReturns400", func(t *testing.T) {
		resp, err := client.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{
				PlanId: plan.Id, RefreshIntervalSeconds: new(10),
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
	})

	t.Run("PutWithInvalidOverridesReturns400", func(t *testing.T) {
		overrides := ct.EntitlementsMap{
			"limits.max_runs": {Type: ct.Numeric, Value: true},
		}
		resp, err := client.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{
				PlanId: plan.Id, EntitlementOverrides: &overrides,
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode())
		require.NotNil(t, resp.JSON400)
		assert.Equal(t, "INVALID_ENTITLEMENTS", resp.JSON400.Errors[0].Code)
	})

	t.Run("PutForOrganizationOfAnotherProductReturns404", func(t *testing.T) {
		otherProduct := createTestProductContext(t)
		otherOrg := otherProduct.CreateOrganization(t, "Other Product Org", nil)

		resp, err := client.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, otherOrg.Id,
			ct.PutOrganizationLicenseJSONRequestBody{PlanId: plan.Id},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})

	t.Run("LifecycleActionsWithoutLicenseReturn404", func(t *testing.T) {
		freshOrg := testProduct.CreateOrganization(t, "No License Org", nil)

		suspendResp, err := client.SuspendOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, freshOrg.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, suspendResp.StatusCode())

		revokeResp, err := client.RevokeOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, freshOrg.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, revokeResp.StatusCode())
	})
}

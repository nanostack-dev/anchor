package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	licverifier "github.com/nanostack-dev/anchor/clients/go/license"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLicenseVerifier builds the offline verifier exactly the way a consumer
// service would: from the signing keys served by the API.
func newLicenseVerifier(
	t *testing.T,
	apiKeyClient *ct.ClientWithResponses,
	productID string,
) *licverifier.Verifier {
	t.Helper()

	resp, err := apiKeyClient.ListLicenseSigningKeysWithResponse(context.Background(), productID)
	require.NoError(t, err, "list signing keys request should not error")
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.NotNil(t, resp.JSON200)
	require.NotEmpty(t, resp.JSON200.Items, "startup hook must have generated a signing key")

	keys := make([]licverifier.PublicKey, 0, len(resp.JSON200.Items))
	hasActive := false
	for _, item := range resp.JSON200.Items {
		keys = append(keys, licverifier.PublicKey{Kid: item.Kid, Key: item.PublicKey})
		if item.Status == ct.LicenseSigningKeyStatusACTIVE {
			hasActive = true
		}
	}
	require.True(t, hasActive, "at least one signing key must be ACTIVE")

	verifier, err := licverifier.NewVerifier(keys)
	require.NoError(t, err, "signing keys served by the API must build a verifier")
	return verifier
}

func issueLicenseToken(
	t *testing.T,
	apiKeyClient *ct.ClientWithResponses,
	productID string,
	organizationID string,
) *ct.IssueLicenseTokenResponse {
	t.Helper()

	resp, err := apiKeyClient.IssueLicenseTokenWithResponse(
		context.Background(), productID, organizationID,
	)
	require.NoError(t, err, "issue license token request should not error")
	return resp
}

func TestLicenseTokenIssuance(t *testing.T) {
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

	verifier := newLicenseVerifier(t, apiKeyClient, testProduct.ProductID)

	t.Run("ActiveLicenseTokenVerifiesEndToEnd", func(t *testing.T) {
		org := testProduct.CreateOrganization(t, "Token Org", nil)
		overrides := ct.EntitlementsMap{
			"limits.max_runs": numericEntitlement(100),
		}
		putResp, err := ownerClient.PutOrganizationLicenseWithResponse(
			ctx, testProduct.ProductID, org.Id,
			ct.PutOrganizationLicenseJSONRequestBody{
				PlanId:               pro.Id,
				EntitlementOverrides: &overrides,
				TokenTtlSeconds:      new(3600),
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, putResp.StatusCode())

		resp := issueLicenseToken(t, apiKeyClient, testProduct.ProductID, org.Id)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.NotEmpty(t, resp.JSON200.Token)

		// refresh_after sits at half the 3600s TTL.
		ttl := resp.JSON200.ExpiresAt.Sub(resp.JSON200.RefreshAfter)
		assert.InDelta(t, (30 * time.Minute).Seconds(), ttl.Seconds(), 5)

		claims, status, verifyErr := verifier.Verify(resp.JSON200.Token)
		require.NoError(t, verifyErr, "token must verify against the published signing keys")
		assert.Equal(t, licverifier.StatusValid, status)
		require.NotNil(t, claims)

		assert.Equal(t, org.Id, claims.OrganizationID)
		assert.Equal(t, testProduct.ProductID, claims.ProductID)
		assert.Equal(t, "pro", claims.PlanKey)
		assert.Equal(t, "ACTIVE", claims.Status)
		assert.Equal(t, 1, claims.SchemaVersion)
		assert.WithinDuration(t, resp.JSON200.ExpiresAt, claims.ExpiresAt, time.Second)
		assert.WithinDuration(t, resp.JSON200.RefreshAfter, claims.RefreshAfter, time.Second)

		// Override wins over the plan default; plan-only values pass through.
		limit, ok := claims.Limit("limits.max_runs")
		require.True(t, ok)
		assert.InDelta(t, 100, limit, 0.0001)
		assert.True(t, claims.HasFeature("api_access"))
		assert.False(t, claims.HasFeature("beta_access"))
		assert.False(t, claims.HasFeature("limits.max_runs"), "numeric is not a feature")

		// A verifier holding a different keypair under the same kid rejects it.
		wrongVerifier, buildErr := licverifier.NewVerifier([]licverifier.PublicKey{{
			Kid: "lsk_2wrongWrongWrongWrongWrong",
			Key: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		}})
		require.NoError(t, buildErr)
		wrongClaims, wrongStatus, wrongErr := wrongVerifier.Verify(resp.JSON200.Token)
		require.Error(t, wrongErr)
		assert.Equal(t, licverifier.StatusInvalid, wrongStatus)
		assert.Nil(t, wrongClaims)
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

		resp := issueLicenseToken(t, fallbackAPIKeyClient, fallbackProduct.ProductID, org.Id)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)

		fallbackVerifier := newLicenseVerifier(
			t, fallbackAPIKeyClient, fallbackProduct.ProductID,
		)
		claims, status, verifyErr := fallbackVerifier.Verify(resp.JSON200.Token)
		require.NoError(t, verifyErr)
		assert.Equal(t, licverifier.StatusValid, status)
		require.NotNil(t, claims)
		assert.Equal(t, "free", claims.PlanKey)
		assert.Equal(t, "ACTIVE", claims.Status)
		assert.Nil(t, claims.GraceUntil)
		limit, ok := claims.Limit("limits.max_runs")
		require.True(t, ok)
		assert.InDelta(t, 3, limit, 0.0001)
	})

	t.Run("NoLicenseAndNoDefaultPlanReturns404", func(t *testing.T) {
		bareProduct := createTestProductContext(t)
		bareAPIKeyClient, _ := bareProduct.CreateAPIKeyClientWithScopes([]string{"license:read"})
		org := bareProduct.CreateOrganization(t, "Unlicensed Org", nil)

		resp := issueLicenseToken(t, bareAPIKeyClient, bareProduct.ProductID, org.Id)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})

	t.Run("SuspendedLicenseIssuesSuspendedToken", func(t *testing.T) {
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

		resp := issueLicenseToken(t, apiKeyClient, testProduct.ProductID, org.Id)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "suspended licenses still get tokens")
		require.NotNil(t, resp.JSON200)

		claims, status, verifyErr := verifier.Verify(resp.JSON200.Token)
		require.NoError(t, verifyErr)
		assert.Equal(t, licverifier.StatusSuspended, status)
		require.NotNil(t, claims)
		assert.Equal(t, "SUSPENDED", claims.Status)
	})

	t.Run("ExpiredWithinGraceIssuesGraceToken", func(t *testing.T) {
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

		resp := issueLicenseToken(t, apiKeyClient, testProduct.ProductID, org.Id)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)

		claims, status, verifyErr := verifier.Verify(resp.JSON200.Token)
		require.NoError(t, verifyErr)
		assert.Equal(t, licverifier.StatusGrace, status)
		require.NotNil(t, claims)
		assert.Equal(t, "GRACE", claims.Status)
		require.NotNil(t, claims.GraceUntil)
		assert.WithinDuration(t, graceUntil, *claims.GraceUntil, time.Second)
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

		resp := issueLicenseToken(t, apiKeyClient, testProduct.ProductID, org.Id)
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

		resp := issueLicenseToken(t, apiKeyClient, testProduct.ProductID, org.Id)
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

		resp := issueLicenseToken(t, apiKeyClient, testProduct.ProductID, org.Id)
		require.Equal(t, http.StatusConflict, resp.StatusCode())
		require.NotNil(t, resp.JSON409)
		assert.Equal(t, "LICENSE_EXPIRED", resp.JSON409.Errors[0].Code)
	})

	t.Run("OrganizationOfAnotherProductReturns404", func(t *testing.T) {
		otherProduct := createTestProductContext(t)
		otherOrg := otherProduct.CreateOrganization(t, "Foreign Org", nil)

		resp := issueLicenseToken(t, apiKeyClient, testProduct.ProductID, otherOrg.Id)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

func TestLicenseTokenAuthz(t *testing.T) {
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

		resp, reqErr := limitedClient.IssueLicenseTokenWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, reqErr)
		itshared.AssertProductAPIKeyInsufficientPermissions(
			t, resp, limitedKeyID, []string{"license:read"}, []string{"organization:read"},
		)
	})

	t.Run("MissingLicenseScopeOnSigningKeysReturns403", func(t *testing.T) {
		limitedClient, limitedKeyID := testProduct.CreateAPIKeyClientWithScopes(
			[]string{"organization:read"},
		)

		resp, reqErr := limitedClient.ListLicenseSigningKeysWithResponse(
			ctx, testProduct.ProductID,
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

		resp, reqErr := otherProductClient.IssueLicenseTokenWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, reqErr)
		assert.Contains(
			t, []int{http.StatusUnauthorized, http.StatusForbidden}, resp.StatusCode(),
			"an API key of another product must not issue tokens for this product",
		)
	})

	t.Run("UnauthenticatedTokenRequestReturns401", func(t *testing.T) {
		unauthenticatedClient, clientErr := ct.NewClientWithResponses(itshared.ServerURL)
		require.NoError(t, clientErr)

		resp, reqErr := unauthenticatedClient.IssueLicenseTokenWithResponse(
			ctx, testProduct.ProductID, org.Id,
		)
		require.NoError(t, reqErr)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
	})
}

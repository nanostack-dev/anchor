package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolEntitlement(value bool) ct.EntitlementValue {
	return ct.EntitlementValue{Type: ct.Boolean, Value: value}
}

func numericEntitlement(value float64) ct.EntitlementValue {
	return ct.EntitlementValue{Type: ct.Numeric, Value: value}
}

func createPlan(
	t *testing.T,
	client *ct.ClientWithResponses,
	productID string,
	body ct.CreatePlanJSONRequestBody,
) ct.PlanResponse {
	t.Helper()

	resp, err := client.CreatePlanWithResponse(context.Background(), productID, body)
	require.NoError(t, err, "create plan request should not error")
	require.Equal(t, http.StatusCreated, resp.StatusCode())
	require.NotNil(t, resp.JSON201, "create plan response body should not be nil")
	return *resp.JSON201
}

func TestPlanCRUD(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	ownerClient := testProduct.OwnerAuthenticatedClient()

	t.Run("CreatePlanWithEntitlements", func(t *testing.T) {
		entitlements := ct.EntitlementsMap{
			"api_access":       boolEntitlement(true),
			"limits.max_runs":  numericEntitlement(25),
			"limits.max_seats": numericEntitlement(5),
		}
		created := createPlan(t, ownerClient, testProduct.ProductID, ct.CreatePlanJSONRequestBody{
			Key:          "pro",
			Name:         "Pro",
			Description:  new("Pro plan"),
			Entitlements: &entitlements,
			IsDefault:    new(false),
		})

		assert.NotEmpty(t, created.Id)
		assert.Equal(t, testProduct.ProductID, created.ProductId)
		assert.Equal(t, "pro", created.Key)
		assert.Equal(t, "Pro", created.Name)
		assert.Equal(t, "Pro plan", *created.Description)
		assert.False(t, created.IsDefault)
		require.Len(t, created.Entitlements, 3)
		assert.Equal(t, boolEntitlement(true), created.Entitlements["api_access"])
		assert.Equal(t, numericEntitlement(25), created.Entitlements["limits.max_runs"])
		assert.NotEmpty(t, created.CreatedAt)
		assert.NotEmpty(t, created.UpdatedAt)

		t.Run("GetPlan", func(t *testing.T) {
			resp, err := ownerClient.GetPlanWithResponse(ctx, testProduct.ProductID, created.Id)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode())
			require.NotNil(t, resp.JSON200)
			assert.Equal(t, created.Id, resp.JSON200.Id)
			assert.Equal(t, created.Entitlements, resp.JSON200.Entitlements)
		})

		t.Run("PatchPlan", func(t *testing.T) {
			updatedEntitlements := ct.EntitlementsMap{
				"api_access":      boolEntitlement(false),
				"limits.max_runs": numericEntitlement(50),
			}
			resp, err := ownerClient.UpdatePlanWithResponse(
				ctx, testProduct.ProductID, created.Id,
				ct.UpdatePlanJSONRequestBody{
					Name:         new("Pro v2"),
					Description:  new("Updated pro plan"),
					Entitlements: &updatedEntitlements,
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode())
			require.NotNil(t, resp.JSON200)
			assert.Equal(t, "Pro v2", resp.JSON200.Name)
			assert.Equal(t, "Updated pro plan", *resp.JSON200.Description)
			assert.Equal(t, "pro", resp.JSON200.Key, "plan key is immutable")
			require.Len(t, resp.JSON200.Entitlements, 2)
			assert.Equal(t, numericEntitlement(50), resp.JSON200.Entitlements["limits.max_runs"])
		})

		t.Run("DeletePlan", func(t *testing.T) {
			deleteResp, err := ownerClient.DeletePlanWithResponse(
				ctx, testProduct.ProductID, created.Id,
			)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode())

			getResp, err := ownerClient.GetPlanWithResponse(ctx, testProduct.ProductID, created.Id)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, getResp.StatusCode())
		})
	})

	t.Run("ListPlans", func(t *testing.T) {
		listProduct := createTestProductContext(t)
		client := listProduct.OwnerAuthenticatedClient()
		createPlan(t, client, listProduct.ProductID, ct.CreatePlanJSONRequestBody{
			Key: "free", Name: "Free",
		})
		createPlan(t, client, listProduct.ProductID, ct.CreatePlanJSONRequestBody{
			Key: "pro", Name: "Pro",
		})

		resp, err := client.ListPlansWithResponse(ctx, listProduct.ProductID)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.Len(t, resp.JSON200.Items, 2)
		assert.Equal(t, "free", resp.JSON200.Items[0].Key)
		assert.Equal(t, "pro", resp.JSON200.Items[1].Key)
	})

	t.Run("GetUnknownPlanReturns404", func(t *testing.T) {
		resp, err := ownerClient.GetPlanWithResponse(
			ctx, testProduct.ProductID, "plan_2ikcVW44U7UtqJHCOTqHuwkgrBb",
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})

	t.Run("DuplicateKeyReturnsTypedError", func(t *testing.T) {
		dupProduct := createTestProductContext(t)
		client := dupProduct.OwnerAuthenticatedClient()
		createPlan(t, client, dupProduct.ProductID, ct.CreatePlanJSONRequestBody{
			Key: "team", Name: "Team",
		})

		resp, err := client.CreatePlanWithResponse(
			ctx, dupProduct.ProductID,
			ct.CreatePlanJSONRequestBody{Key: "team", Name: "Team Again"},
		)
		require.NoError(t, err)
		itshared.AssertAnchorBadRequestError(
			t, resp,
			"PLAN_KEY_DUPLICATE",
			"A plan with this key already exists in the product",
			map[string]any{"plan_key": "team", "product_id": dupProduct.ProductID},
		)
	})

	t.Run("InvalidEntitlementsRejected", func(t *testing.T) {
		badEntitlements := ct.EntitlementsMap{
			"limits.max_runs": {Type: ct.Numeric, Value: "not-a-number"},
		}
		resp, err := ownerClient.CreatePlanWithResponse(
			ctx, testProduct.ProductID,
			ct.CreatePlanJSONRequestBody{
				Key: "bad-entitlements", Name: "Bad", Entitlements: &badEntitlements,
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode())
		require.NotNil(t, resp.JSON400)
		assert.Equal(t, "INVALID_ENTITLEMENTS", resp.JSON400.Errors[0].Code)
	})

	t.Run("InvalidEntitlementKeyRejected", func(t *testing.T) {
		badEntitlements := ct.EntitlementsMap{
			"Not A Valid Key": boolEntitlement(true),
		}
		resp, err := ownerClient.CreatePlanWithResponse(
			ctx, testProduct.ProductID,
			ct.CreatePlanJSONRequestBody{
				Key: "bad-key", Name: "Bad Key", Entitlements: &badEntitlements,
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode())
		require.NotNil(t, resp.JSON400)
		assert.Equal(t, "INVALID_ENTITLEMENTS", resp.JSON400.Errors[0].Code)
	})
}

func TestPlanDefaultUniqueness(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	client := testProduct.OwnerAuthenticatedClient()

	first := createPlan(t, client, testProduct.ProductID, ct.CreatePlanJSONRequestBody{
		Key: "free", Name: "Free", IsDefault: new(true),
	})
	require.True(t, first.IsDefault)

	// Creating a second default plan clears the first one's flag.
	second := createPlan(t, client, testProduct.ProductID, ct.CreatePlanJSONRequestBody{
		Key: "pro", Name: "Pro", IsDefault: new(true),
	})
	require.True(t, second.IsDefault)

	firstReloaded, err := client.GetPlanWithResponse(ctx, testProduct.ProductID, first.Id)
	require.NoError(t, err)
	require.NotNil(t, firstReloaded.JSON200)
	assert.False(t, firstReloaded.JSON200.IsDefault, "previous default plan must be cleared")

	// Patching the first plan back to default clears the second.
	patchResp, err := client.UpdatePlanWithResponse(
		ctx, testProduct.ProductID, first.Id,
		ct.UpdatePlanJSONRequestBody{IsDefault: new(true)},
	)
	require.NoError(t, err)
	require.NotNil(t, patchResp.JSON200)
	assert.True(t, patchResp.JSON200.IsDefault)

	secondReloaded, err := client.GetPlanWithResponse(ctx, testProduct.ProductID, second.Id)
	require.NoError(t, err)
	require.NotNil(t, secondReloaded.JSON200)
	assert.False(t, secondReloaded.JSON200.IsDefault, "default flag must move to the patched plan")
}

func TestPlanDeleteInUse(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)
	client := testProduct.OwnerAuthenticatedClient()
	plan := createPlan(t, client, testProduct.ProductID, ct.CreatePlanJSONRequestBody{
		Key: "pro", Name: "Pro",
	})
	org := testProduct.CreateOrganization(t, "License Org", nil)

	putResp, err := client.PutOrganizationLicenseWithResponse(
		ctx, testProduct.ProductID, org.Id,
		ct.PutOrganizationLicenseJSONRequestBody{PlanId: plan.Id},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, putResp.StatusCode())

	deleteResp, err := client.DeletePlanWithResponse(ctx, testProduct.ProductID, plan.Id)
	require.NoError(t, err)
	itshared.AssertAnchorBadRequestError(
		t, deleteResp,
		"PLAN_IN_USE",
		"Plan "+plan.Id+" cannot be deleted because 1 license(s) still reference it",
		map[string]any{"plan_id": plan.Id, "license_count": float64(1)},
	)
}

func TestPlanEndpointsRequirePlatformBearer(t *testing.T) {
	ctx := context.Background()

	testProduct := createTestProductContext(t)

	unauthenticatedClient, err := ct.NewClientWithResponses(itshared.ServerURL)
	require.NoError(t, err)

	t.Run("UnauthenticatedCreateReturns401", func(t *testing.T) {
		resp, reqErr := unauthenticatedClient.CreatePlanWithResponse(
			ctx, testProduct.ProductID,
			ct.CreatePlanJSONRequestBody{Key: "pro", Name: "Pro"},
		)
		require.NoError(t, reqErr)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
	})

	t.Run("UnauthenticatedListReturns401", func(t *testing.T) {
		resp, reqErr := unauthenticatedClient.ListPlansWithResponse(ctx, testProduct.ProductID)
		require.NoError(t, reqErr)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
	})

	t.Run("ProductAPIKeyCannotManagePlans", func(t *testing.T) {
		// Plans are platform-admin surface: even an all-scope product API key
		// must not reach them.
		resp, reqErr := testProduct.AllScopeAPIKeyClient().CreatePlanWithResponse(
			ctx, testProduct.ProductID,
			ct.CreatePlanJSONRequestBody{Key: "pro", Name: "Pro"},
		)
		require.NoError(t, reqErr)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
	})
}

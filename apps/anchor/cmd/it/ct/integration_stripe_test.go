package ct_test

import (
	"encoding/json"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/integration"
	stripeprovider "anchor/internal/integration/provider/stripe"
)

const (
	stripeFixtureAccount = "acct_Fixture123"
	stripeFixtureKey     = "sk_test_fixture"
	stripeFixtureWebhook = "whsec_encryptedContractFixture"
)

// The fixture deliberately omits the return URL. Connection verification therefore
// stops before any Stripe request, while HTTP CRUD still exercises encryption.
func createStripeContractInstance(t *testing.T, tc webhookSecretTestCtx) *ct.IntegrationInstanceResponse {
	t.Helper()
	body := ct.CreateIntegrationInstanceJSONRequestBody{}
	require.NoError(t, body.FromStripeIntegrationInstanceCreateRequest(ct.StripeIntegrationInstanceCreateRequest{
		ProviderType: ct.StripeIntegrationInstanceCreateRequestProviderTypeSTRIPE,
		Config: &ct.StripeIntegrationConfig{
			AuthMethod: new(ct.APIKEY), AccountId: new(stripeFixtureAccount),
			ApiKey: new(stripeFixtureKey), WebhookSecret: new(stripeFixtureWebhook),
		},
	}))
	response, err := tc.product.OwnerAuthenticatedClient().CreateIntegrationInstanceWithResponse(
		t.Context(), tc.product.ProductID, body,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, response.StatusCode(), string(response.Body))
	require.NotNil(t, response.JSON201)
	return response.JSON201
}

func stripeStoredConfig(t *testing.T, tc webhookSecretTestCtx, id string) stripeprovider.Config {
	t.Helper()
	found, err := IntegrationRepo.FindByID(t.Context(), tc.tenantID, id)
	require.NoError(t, err)
	require.True(t, found.IsPresent())
	instance := found.Value()
	require.Nil(t, instance.WebhookSecret, "Stripe secret must only live in encrypted provider configuration")
	require.NotContains(t, string(instance.ConfigJSON), stripeFixtureKey)
	require.NotContains(t, string(instance.ConfigJSON), stripeFixtureWebhook)
	var cfg stripeprovider.Config
	require.NoError(t, json.Unmarshal(instance.ConfigJSON, &cfg))
	require.True(t, secrets.IsVersionedEncryptedSecret(cfg.APIKey))
	require.True(t, secrets.IsVersionedEncryptedSecret(cfg.WebhookSecret))
	return cfg
}

func assertStripePublicConfig(t *testing.T, instance *ct.IntegrationInstanceResponse) {
	t.Helper()
	public, err := instance.PublicConfig.AsStripeIntegrationPublicConfig()
	require.NoError(t, err)
	require.Equal(t, ct.IntegrationProviderTypeSTRIPE, instance.ProviderType)
	require.Equal(t, ct.APIKEY, public.AuthMethod)
	require.Equal(t, ct.StripeIntegrationPublicConfigModeSandbox, public.Mode)
	require.Equal(t, stripeFixtureAccount, *public.AccountId)
	require.True(t, public.ApiKeyConfigured)
	require.True(t, public.WebhookSecretConfigured)
	serialized, err := json.Marshal(instance)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), stripeFixtureKey)
	require.NotContains(t, string(serialized), stripeFixtureWebhook)
}

func TestStripeIntegrationCRUDEncryptsCredentialsAndPreservesBlankSecrets(t *testing.T) {
	t.Parallel()
	tc := newWebhookSecretTestCtx(t)
	instance := createStripeContractInstance(t, tc)
	assertStripePublicConfig(t, instance)
	before := stripeStoredConfig(t, tc, instance.Id)
	cfg := ct.IntegrationProviderConfig{}
	require.NoError(t, cfg.FromStripeIntegrationConfig(ct.StripeIntegrationConfig{
		AuthMethod: new(ct.APIKEY), AccountId: new(stripeFixtureAccount),
		ApiKey: new(""), WebhookSecret: new(""),
	}))
	response := updateInstance(t, tc, instance.Id, ct.UpdateIntegrationInstanceJSONRequestBody{Config: &cfg})
	require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	require.NotNil(t, response.JSON200)
	assertStripePublicConfig(t, response.JSON200)
	after := stripeStoredConfig(t, tc, instance.Id)
	require.Equal(t, before.APIKey, after.APIKey)
	require.Equal(t, before.WebhookSecret, after.WebhookSecret)
	fetched := getIntegrationInstance(t, tc.product, instance.Id)
	assertStripePublicConfig(t, fetched)
	audit := listIntegrationAuditLogs(t, tc.product, instance.Id)
	serialized, err := json.Marshal(audit)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), stripeFixtureKey)
	require.NotContains(t, string(serialized), stripeFixtureWebhook)
}

func TestStripeIntegrationRefusesLiveCredentialsBeforeStorage(t *testing.T) {
	t.Parallel()
	tc := newWebhookSecretTestCtx(t)
	body := ct.CreateIntegrationInstanceJSONRequestBody{}
	require.NoError(t, body.FromStripeIntegrationInstanceCreateRequest(ct.StripeIntegrationInstanceCreateRequest{
		ProviderType: ct.StripeIntegrationInstanceCreateRequestProviderTypeSTRIPE,
		Config:       &ct.StripeIntegrationConfig{AuthMethod: new(ct.APIKEY), ApiKey: new("sk_live_rejectedFixture")},
	}))
	response, err := tc.product.OwnerAuthenticatedClient().CreateIntegrationInstanceWithResponse(
		t.Context(), tc.product.ProductID, body,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, response.StatusCode(), string(response.Body))
	require.NotContains(t, string(response.Body), "sk_live_rejectedFixture")
	instances := listIntegrationInstances(t, tc.product)
	require.Empty(t, instances.Items)
}

func TestStripeIntegrationAndBillingRespectTenantAndPlatformAuthentication(t *testing.T) {
	t.Parallel()
	owner := newWebhookSecretTestCtx(t)
	foreign := newWebhookSecretTestCtx(t)
	instance := createStripeContractInstance(t, owner)
	read, err := foreign.product.OwnerAuthenticatedClient().GetIntegrationInstanceWithResponse(
		t.Context(), foreign.product.ProductID, instance.Id,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, read.StatusCode())
	update, err := foreign.product.OwnerAuthenticatedClient().UpdateIntegrationInstanceWithResponse(
		t.Context(),
		foreign.product.ProductID, instance.Id, ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(false)},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, update.StatusCode())
	crossTenant, err := foreign.product.OwnerAuthenticatedClient().GetStripeBillingStateWithResponse(
		t.Context(), owner.product.ProductID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, crossTenant.StatusCode())
	productKey, err := owner.product.AllScopeAPIKeyClient().GetStripeBillingStateWithResponse(
		t.Context(), owner.product.ProductID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, productKey.StatusCode())
	write, err := owner.product.AllScopeAPIKeyClient().CreateStripeBillingPriceWithResponse(
		t.Context(), owner.product.ProductID, stripePriceFixture(),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, write.StatusCode())
}

func stripePriceFixture() ct.CreateStripeBillingPriceJSONRequestBody {
	return ct.CreateStripeBillingPriceJSONRequestBody{
		Name: "Sandbox contract price", TemplateId: "ltpl_fixtureTemplate", Amount: 100,
		Currency: ct.StripeBillingCreatePriceRequestCurrencyUsd,
		Interval: ct.StripeBillingCreatePriceRequestIntervalMonth,
	}
}

func TestStripeBillingRequiresConnectedIntegrationBeforeReadsOrMutations(t *testing.T) {
	t.Parallel()
	tc := newWebhookSecretTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()
	missing, err := client.GetStripeBillingStateWithResponse(t.Context(), tc.product.ProductID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, missing.StatusCode())
	instance := createStripeContractInstance(t, tc)
	disconnected, err := client.GetStripeBillingStateWithResponse(t.Context(), tc.product.ProductID)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, disconnected.StatusCode())
	mutation, err := client.CreateStripeBillingPriceWithResponse(
		t.Context(),
		tc.product.ProductID,
		stripePriceFixture(),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, mutation.StatusCode())
	disabled := updateInstance(t, tc, instance.Id, ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(false)})
	require.Equal(t, http.StatusOK, disabled.StatusCode())
	unavailable, err := client.GetStripeBillingStateWithResponse(t.Context(), tc.product.ProductID)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, unavailable.StatusCode())
}

func TestConnectionVerificationCASPreservesChangedConfigurationAndPause(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		update func(*integration.Instance)
	}{
		{"configuration changed", func(instance *integration.Instance) {
			instance.ConfigJSON = json.RawMessage(`{"auth_method":"API_KEY","account_id":"acct_New"}`)
		}},
		{"schema version changed", func(instance *integration.Instance) { instance.ConfigVersion++ }},
		{"disabled", func(instance *integration.Instance) {
			instance.IsEnabled = false
			instance.Status = integration.StatusInactive
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tc := newWebhookSecretTestCtx(t)
			verified := seedActiveStripeWebhookInstance(t, tc)
			changed := verified
			test.update(&changed)
			current, err := IntegrationRepo.Update(t.Context(), tc.tenantID, changed)
			require.NoError(t, err)
			result, err := IntegrationRepo.CompareAndSetVerificationStatusInternal(t.Context(), verified,
				integration.StatusError, new("stale verification error"))
			require.NoError(t, err)
			require.True(t, result.IsAbsent(), "stale verification must not match the current row")
			persisted, err := IntegrationRepo.FindByID(t.Context(), tc.tenantID, verified.ID)
			require.NoError(t, err)
			require.True(t, persisted.IsPresent())
			require.JSONEq(t, string(current.ConfigJSON), string(persisted.Value().ConfigJSON))
			require.Equal(t, current.ConfigVersion, persisted.Value().ConfigVersion)
			require.Equal(t, current.IsEnabled, persisted.Value().IsEnabled)
			require.Equal(t, current.Status, persisted.Value().Status)
			require.Nil(t, persisted.Value().LastError)
		})
	}
}

func TestConnectionVerificationCASChangesOnlyStatusWithinExactTenantProductScope(t *testing.T) {
	t.Parallel()
	tc := newWebhookSecretTestCtx(t)
	verified := seedActiveStripeWebhookInstance(t, tc)
	wrongScope := verified
	wrongScope.PlatformTenantID = "tenant_foreign"
	refused, err := IntegrationRepo.CompareAndSetVerificationStatusInternal(t.Context(), wrongScope,
		integration.StatusError, new("cross-tenant verification"))
	require.NoError(t, err)
	require.True(t, refused.IsAbsent())
	updated, err := IntegrationRepo.CompareAndSetVerificationStatusInternal(t.Context(), verified,
		integration.StatusError, new("connection rejected"))
	require.NoError(t, err)
	require.True(t, updated.IsPresent())
	require.Equal(t, integration.StatusError, updated.Value().Status)
	require.Equal(t, "connection rejected", *updated.Value().LastError)
	require.JSONEq(t, string(verified.ConfigJSON), string(updated.Value().ConfigJSON))
}

func TestStripeBillingMissingPathResourcesReturn404AndBodySelectionsReturn400(t *testing.T) {
	t.Parallel()
	tc := newWebhookSecretTestCtx(t)
	seedActiveStripeWebhookInstance(t, tc)
	client := tc.product.OwnerAuthenticatedClient()
	missingOrganizationID := "org_" + ksuid.New().String()
	archive, err := client.ArchiveStripeBillingPriceWithResponse(t.Context(), tc.product.ProductID, "price_missing")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, archive.StatusCode(), string(archive.Body))
	sync, err := client.SyncStripeBillingOrganizationWithResponse(
		t.Context(),
		tc.product.ProductID,
		missingOrganizationID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, sync.StatusCode(), string(sync.Body))
	portal, err := client.CreateStripeBillingPortalWithResponse(
		t.Context(),
		tc.product.ProductID,
		missingOrganizationID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, portal.StatusCode(), string(portal.Body))
	organization := tc.product.CreateOrganization(t, "Body selection fixture", nil)
	checkout, err := client.CreateStripeBillingCheckoutWithResponse(t.Context(), tc.product.ProductID, organization.Id,
		ct.CreateStripeBillingCheckoutJSONRequestBody{PriceId: "price_missing"})
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, checkout.StatusCode(), string(checkout.Body))
}

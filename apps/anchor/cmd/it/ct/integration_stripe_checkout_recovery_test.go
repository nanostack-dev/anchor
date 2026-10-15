package ct_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"anchor/internal/domain/integration"
	"anchor/internal/stripebilling/billing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
)

type checkoutRecoveryFixture struct {
	context        webhookSecretTestCtx
	instance       integration.Instance
	organizationID string
	priceID        string
	intent         string
	trial          int
}

func seedCheckoutRecoveryFixture(t *testing.T, intent string, trial int, customerID string) checkoutRecoveryFixture {
	t.Helper()
	tc := newWebhookSecretTestCtx(t)
	instance := seedActiveStripeWebhookInstance(t, tc)
	deferStripeFixtureJobs(t, instance.ID)
	organization := tc.product.CreateOrganization(t, "Checkout recovery "+ksuid.New().String(), nil)
	state := billing.NewStoredState(stripeFixtureAccount, tc.product.ProductID)
	priceID := ksuid.New().String()
	state.Prices[priceID] = billing.Price{
		ID: priceID, Name: "Recovery price", Active: true, Amount: 1500, Currency: "usd", Interval: "month",
		TemplateID: ksuid.New().String(), StripePriceID: "price_" + ksuid.New().String(),
		StripeProductID: "prod_" + ksuid.New().String(),
	}
	body, err := json.Marshal(state)
	require.NoError(t, err)
	var stored map[string]any
	require.NoError(t, json.Unmarshal(body, &stored))
	stored["organizations"] = map[string]any{organization.Id: map[string]any{
		"id": organization.Id, "name": organization.Name, "customer_id": customerID,
		"checkout_intent": intent, "checkout_id": "", "checkout_price": priceID, "checkout_trial": trial,
	}}
	body, err = json.Marshal(stored)
	require.NoError(t, err)
	_, err = testDB.ExecContext(t.Context(), `INSERT INTO stripe_billing_states
		(integration_instance_id,platform_tenant_id,product_id,state_json) VALUES($1,$2,$3,$4::jsonb)`,
		instance.ID, tc.tenantID, tc.product.ProductID, string(body))
	require.NoError(t, err)
	return checkoutRecoveryFixture{
		context: tc, instance: instance, organizationID: organization.Id,
		priceID: priceID, intent: intent, trial: trial,
	}
}

func (fixture checkoutRecoveryFixture) requireUnchanged(t *testing.T) {
	t.Helper()
	persisted := readStripeFinancialState(t, fixture.instance)
	row := persisted.Organizations[fixture.organizationID]
	require.Equal(t, fixture.intent, row.CheckoutIntent, "refusal must preserve the submitted request identity")
	require.Empty(t, row.CheckoutID)
	require.Equal(t, fixture.priceID, row.CheckoutPrice)
	require.Equal(t, fixture.trial, row.CheckoutTrial)
	require.True(t, persisted.Prices[fixture.priceID].Active)
}

func TestStripeArchivePriceRefusesAnUnresolvedCheckoutThroughHTTP(t *testing.T) {
	t.Parallel()
	fixture := seedCheckoutRecoveryFixture(t, ksuid.New().String(), 7, "cus_"+ksuid.New().String())
	response, err := fixture.context.product.OwnerAuthenticatedClient().ArchiveStripeBillingPriceWithResponse(
		t.Context(), fixture.context.product.ProductID, fixture.priceID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, response.StatusCode(), string(response.Body))
	require.Contains(t, string(response.Body), "recover the pending checkout")
	fixture.requireUnchanged(t)
}

func TestStripeCheckoutRefusesUnsafeRecoveryThroughHTTP(t *testing.T) {
	t.Parallel()
	for name, request := range map[string]struct {
		intent      func(t *testing.T) string
		customerID  string
		changePrice bool
		changeTrial bool
	}{
		"different price":  {intent: func(*testing.T) string { return ksuid.New().String() }, customerID: "linked", changePrice: true},
		"different trial":  {intent: func(*testing.T) string { return ksuid.New().String() }, customerID: "linked", changeTrial: true},
		"missing customer": {intent: func(*testing.T) string { return ksuid.New().String() }},
		"invalid intent":   {intent: func(*testing.T) string { return "legacy-unknown-checkout" }, customerID: "linked"},
		"future intent": {intent: func(t *testing.T) string {
			value, err := ksuid.NewRandomWithTime(time.Now().Add(time.Hour))
			require.NoError(t, err)
			return value.String()
		}, customerID: "linked"},
		"expired intent": {intent: func(t *testing.T) string {
			value, err := ksuid.NewRandomWithTime(time.Now().Add(-24 * time.Hour))
			require.NoError(t, err)
			return value.String()
		}, customerID: "linked"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			customerID := ""
			if request.customerID != "" {
				customerID = "cus_" + ksuid.New().String()
			}
			fixture := seedCheckoutRecoveryFixture(t, request.intent(t), 7, customerID)
			body := ct.CreateStripeBillingCheckoutJSONRequestBody{PriceId: fixture.priceID, TrialDays: &fixture.trial}
			if request.changePrice {
				body.PriceId = ksuid.New().String()
			}
			if request.changeTrial {
				body.TrialDays = new(8)
			}
			response, err := fixture.context.product.OwnerAuthenticatedClient().CreateStripeBillingCheckoutWithResponse(
				t.Context(), fixture.context.product.ProductID, fixture.organizationID, body,
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusConflict, response.StatusCode(), string(response.Body))
			fixture.requireUnchanged(t)
		})
	}
}

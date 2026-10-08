//nolint:testpackage // Tests inspect private persisted records to verify recovery and webhook isolation.
package stripeprototype

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stripeRequest struct {
	method      string
	path        string
	params      map[string]string
	idempotency string
}

type fakeStripe struct {
	current  map[string]any
	requests []stripeRequest
	respond  func(string, string, map[string]string, string) (any, error)
}

func (*fakeStripe) Account(context.Context) (Account, error) {
	return Account{ID: "acct_expected", Name: "New Business", Mode: Sandbox}, nil
}

func (s *fakeStripe) Request(
	_ context.Context,
	method, path string,
	params map[string]string,
	idempotency string,
	result any,
) error {
	s.requests = append(s.requests, stripeRequest{method: method, path: path, params: params, idempotency: idempotency})
	var response any
	var err error
	switch {
	case s.respond != nil:
		response, err = s.respond(method, path, params, idempotency)
		if err != nil {
			return err
		}
	case path == "/v1/subscriptions":
		items := []any{}
		if s.current != nil {
			items = append(items, s.current)
		}
		response = map[string]any{"data": items}
	case strings.HasPrefix(path, "/v1/subscriptions/"):
		response = s.current
	default:
		return fmt.Errorf("unexpected Stripe request: %s %s", method, path)
	}
	data, err := json.Marshal(response)
	if err != nil || result == nil {
		return err
	}
	return json.Unmarshal(data, result)
}

type templateApplication struct {
	organizationID string
	templateID     string
}

type fakeAnchor struct {
	snapshot     AnchorSnapshot
	applications []templateApplication
	applyError   error
}

func (a *fakeAnchor) Snapshot(context.Context) (AnchorSnapshot, error) {
	return a.snapshot, nil
}

func (a *fakeAnchor) ApplyTemplate(_ context.Context, organizationID, templateID string) error {
	a.applications = append(a.applications, templateApplication{organizationID: organizationID, templateID: templateID})
	if a.applyError != nil {
		return a.applyError
	}
	for index := range a.snapshot.Organizations {
		if a.snapshot.Organizations[index].ID == organizationID {
			a.snapshot.Organizations[index].TemplateID = templateID
		}
	}
	return nil
}

type serviceWorld struct {
	service      *Service
	store        *Store
	stripe       *fakeStripe
	anchor       *fakeAnchor
	config       Config
	organization string
	freeTemplate string
	paidTemplate string
	price        Price
}

func newServiceWorld(t *testing.T) *serviceWorld {
	t.Helper()
	productID := "prd_" + ksuid.New().String()
	organizationID := "org_" + ksuid.New().String()
	freeTemplate := "ltpl_" + ksuid.New().String()
	paidTemplate := "ltpl_" + ksuid.New().String()
	price := Price{ID: ksuid.New().String(), Name: "Pro monthly", Amount: 2500,
		Currency: PriceCurrencyUsd, Interval: PriceIntervalMonth, Active: true,
		TemplateID: paidTemplate, StripePriceID: "price_paid", StripeProductID: "prod_paid"}
	store, err := OpenStore(filepath.Join(t.TempDir(), "billing.json"), "acct_expected", productID, freeTemplate)
	require.NoError(t, err)
	w := &serviceWorld{store: store}
	t.Cleanup(func() { require.NoError(t, w.store.Close()) })
	err = store.Update(func(state *StoredState) error {
		state.Prices[price.ID] = price
		state.Organizations[organizationID] = organizationRecord{
			ID: organizationID, Name: "Test organization", CustomerID: "cus_expected", SubscriptionID: "sub_expected",
			TemplateID: freeTemplate, Status: "free", LicenseValues: map[string]any{"seat_limit": float64(7)},
		}
		return nil
	})
	require.NoError(t, err)
	anchor := &fakeAnchor{snapshot: AnchorSnapshot{
		Product: Product{ID: productID, Name: "Anchor test product"},
		Templates: []Template{
			{ID: freeTemplate, Name: "Free", Values: map[string]any{"seat_limit": float64(2)}},
			{ID: paidTemplate, Name: "Pro", Values: map[string]any{"seat_limit": float64(10)}},
		},
		Organizations: []AnchorOrganization{{ID: organizationID, Name: "Test organization",
			TemplateID: freeTemplate, LicenseValues: map[string]any{"seat_limit": float64(7)}}},
	}}
	stripe := &fakeStripe{current: subscriptionPayload("sub_expected", "cus_expected", "active", "price_paid")}
	stored, err := store.Snapshot()
	require.NoError(t, err)
	stripe.current["metadata"] = map[string]string{"anchor_prototype_id": stored.InstallationID,
		"anchor_organization_id": organizationID, "anchor_product_id": productID}
	config := Config{ExpectedAccountID: "acct_expected", ProductID: productID,
		ReturnURL: "http://127.0.0.1:4247", WebhookSecret: "whsec_test_signature_secret"}
	service, err := NewService(config, stripe, anchor, store)
	require.NoError(t, err)
	w.service, w.stripe, w.anchor, w.config = service, stripe, anchor, config
	w.organization, w.freeTemplate, w.paidTemplate, w.price = organizationID, freeTemplate, paidTemplate, price
	return w
}

func (w *serviceWorld) setSubscription(status, price string) {
	metadata := w.stripe.current["metadata"]
	w.stripe.current = subscriptionPayload("sub_expected", "cus_expected", status, price)
	w.stripe.current["metadata"] = metadata
}

func subscriptionPayload(id, customer, status, price string) map[string]any {
	return map[string]any{"id": id, "object": "subscription", "customer": customer, "status": status,
		"livemode": false, "cancel_at_period_end": false, "cancel_at": nil, "pending_update": nil,
		"items": map[string]any{"data": []any{map[string]any{
			"id": "si_expected", "current_period_end": time.Now().Add(30 * 24 * time.Hour).Unix(),
			"price": map[string]any{"id": price},
		}}},
	}
}

func signedEvent(t *testing.T, secret, eventID, eventType string, object map[string]any) ([]byte, string) {
	t.Helper()
	timestamp := time.Now().Unix()
	body, err := json.Marshal(map[string]any{"id": eventID, "object": "event", "type": eventType,
		"api_version": StripeVersion, "created": timestamp, "livemode": false,
		"data": map[string]any{"object": object}})
	require.NoError(t, err)
	return body, signEventBody(t, secret, timestamp, body)
}

func signEventBody(t *testing.T, secret string, timestamp int64, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write(append([]byte(strconv.FormatInt(timestamp, 10)+"."), body...))
	require.NoError(t, err)
	return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

func readStoredState(path string) (StoredState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StoredState{}, err
	}
	var state StoredState
	err = json.Unmarshal(data, &state)
	return state, err
}

func (w *serviceWorld) restart(t *testing.T) {
	t.Helper()
	path := w.store.path
	require.NoError(t, w.store.Close())
	store, err := OpenStore(path, w.config.ExpectedAccountID, w.config.ProductID, w.freeTemplate)
	require.NoError(t, err)
	w.store = store
	w.service, err = NewService(w.config, w.stripe, w.anchor, store)
	require.NoError(t, err)
}

func TestFirstRunStateSerializesEmptyCollectionsAsArrays(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	w.anchor.snapshot.Organizations = nil
	w.anchor.snapshot.Templates = nil
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		state.Prices = map[string]Price{}
		state.Organizations = map[string]organizationRecord{}
		state.Events = map[string]eventRecord{}
		return nil
	}))
	state, err := w.service.State(t.Context())
	require.NoError(t, err)
	body, err := json.Marshal(state)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	for _, field := range []string{"prices", "events", "organizations", "templates"} {
		assert.JSONEq(t, "[]", string(fields[field]), "%s must satisfy the required array contract", field)
	}
}

func TestHTTPCheckoutAcceptsAnchorOrganizationPrefixAndRejectsPathInjection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		injected string
	}{
		{name: "prefixed organization"},
		{name: "shell characters", injected: "; echo untrusted"},
		{name: "path traversal", injected: "/../../prices"},
		{name: "query injection", injected: "?customer=cus_foreign"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			w := newServiceWorld(t)
			require.NoError(t, w.store.Update(func(state *StoredState) error {
				organization := state.Organizations[w.organization]
				organization.SubscriptionID = ""
				state.Organizations[w.organization] = organization
				return nil
			}))
			w.stripe.respond = func(_ string, path string, _ map[string]string, _ string) (any, error) {
				switch path {
				case "/v1/subscriptions":
					return map[string]any{"data": []any{}}, nil
				case "/v1/checkout/sessions":
					return map[string]any{"id": "cs_prefixed", "url": "https://checkout.stripe.com/prefixed"}, nil
				default:
					return nil, fmt.Errorf("unexpected request %s", path)
				}
			}
			specification, err := os.ReadFile(filepath.Join("..", "..", "cmd", "stripe-prototype", "openapi.yaml"))
			require.NoError(t, err)
			handler, err := NewHTTPHandler(w.service, specification)
			require.NoError(t, err)
			body, err := json.Marshal(CheckoutRequest{PriceID: w.price.ID})
			require.NoError(t, err)
			target := w.config.ReturnURL + "/api/organizations/" + url.PathEscape(
				w.organization+test.injected,
			) + "/checkout"
			request := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", w.config.ReturnURL)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if test.injected != "" {
				assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, response.Code)
				assert.Empty(t, w.stripe.requests)
				assert.Empty(t, w.anchor.applications)
				return
			}
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var result URLResponse
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			assert.Equal(t, "https://checkout.stripe.com/prefixed", result.URL)
			require.Len(t, w.stripe.requests, 2)
			checkout := w.stripe.requests[1]
			assert.Equal(t, w.organization, checkout.params["client_reference_id"])
			assert.Equal(t, w.organization, checkout.params["subscription_data[metadata][anchor_organization_id]"])
			assert.Equal(t, w.config.ProductID, checkout.params["subscription_data[metadata][anchor_product_id]"])
		})
	}
}

func TestCheckoutLostResponsePreservesIntentUntilRecovered(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"price", "trial"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			w := newServiceWorld(t)
			alternate := w.price
			alternate.ID, alternate.StripePriceID = ksuid.New().String(), "price_alternate"
			require.NoError(t, w.store.Update(func(state *StoredState) error {
				state.Prices[alternate.ID] = alternate
				organization := state.Organizations[w.organization]
				organization.SubscriptionID = ""
				state.Organizations[w.organization] = organization
				return nil
			}))
			var remoteIntent string
			checkoutCalls := 0
			w.stripe.respond = func(_ string, path string, _ map[string]string, intent string) (any, error) {
				switch path {
				case "/v1/subscriptions":
					return map[string]any{"data": []any{}}, nil
				case "/v1/checkout/sessions":
					checkoutCalls++
					if remoteIntent == "" {
						remoteIntent = intent
						return nil, errors.New("checkout response was lost after Stripe created the session")
					}
					assert.Equal(t, remoteIntent, intent)
					return map[string]any{"id": "cs_recovered", "url": "https://checkout.stripe.com/recovered"}, nil
				default:
					return nil, fmt.Errorf("unexpected request %s", path)
				}
			}
			original := CheckoutRequest{PriceID: w.price.ID}
			_, err := w.service.Checkout(t.Context(), w.organization, original)
			require.Error(t, err)
			w.restart(t)
			changed := original
			if change == "price" {
				changed.PriceID = alternate.ID
			} else {
				changed.TrialDays = 7
			}
			_, err = w.service.Checkout(t.Context(), w.organization, changed)
			require.ErrorIs(t, err, ErrConflict)
			assert.Equal(t, 1, checkoutCalls)
			state, err := w.store.Snapshot()
			require.NoError(t, err)
			assert.Equal(t, remoteIntent, state.Organizations[w.organization].CheckoutIntent)
			assert.Equal(t, original.PriceID, state.Organizations[w.organization].CheckoutPrice)
			result, err := w.service.Checkout(t.Context(), w.organization, original)
			require.NoError(t, err)
			assert.Equal(t, "https://checkout.stripe.com/recovered", result.URL)
			assert.Equal(t, 2, checkoutCalls)
		})
	}
}

func TestCreatePriceRecoversProductAcrossRestartAndChangedPriceName(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"price request", "product response"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			w := newServiceWorld(t)
			productCalls, priceCalls := 0, 0
			var productIntent, productName string
			w.stripe.respond = func(_ string, path string, params map[string]string, intent string) (any, error) {
				switch path {
				case "/v1/products":
					productCalls++
					if productIntent == "" {
						productIntent, productName = intent, params["name"]
						if failure == "product response" {
							return nil, errors.New("product response was lost after Stripe created the product")
						}
					} else {
						assert.Equal(t, productIntent, intent)
						assert.Equal(t, productName, params["name"])
					}
					return map[string]any{"id": "prod_recovered"}, nil
				case "/v1/prices":
					priceCalls++
					assert.Equal(t, "prod_recovered", params["product"])
					if failure == "price request" && priceCalls == 1 {
						return nil, errors.New("price request failed after product creation")
					}
					return map[string]any{"id": "price_recovered"}, nil
				default:
					return nil, fmt.Errorf("unexpected request %s", path)
				}
			}
			request := CreatePriceRequest{Name: "Original monthly", TemplateID: w.paidTemplate, Amount: 2500,
				Currency: CreatePriceRequestCurrencyUsd, Interval: CreatePriceRequestIntervalMonth}
			_, err := w.service.CreatePrice(t.Context(), request)
			require.Error(t, err)
			state, err := w.store.Snapshot()
			require.NoError(t, err)
			assert.Equal(t, productName, state.ProductNames[w.paidTemplate])
			if failure == "price request" {
				assert.Equal(t, "prod_recovered", state.StripeProducts[w.paidTemplate])
			}
			w.restart(t)
			w.anchor.snapshot.Templates[1].Name = "Renamed template"
			request.Name, request.Amount = "Replacement monthly", 3000
			price, err := w.service.CreatePrice(t.Context(), request)
			require.NoError(t, err)
			assert.Equal(t, "prod_recovered", price.StripeProductID)
			assert.Equal(t, "Replacement monthly", price.Name)
			if failure == "price request" {
				assert.Equal(t, 1, productCalls)
			} else {
				assert.Equal(t, 2, productCalls)
			}
		})
	}
}

func TestCheckoutCompletingDuringRecoveryCannotCreateAnotherSession(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		organization := state.Organizations[w.organization]
		organization.SubscriptionID = ""
		organization.CheckoutID, organization.CheckoutPrice = "cs_completed", w.price.ID
		state.Organizations[w.organization] = organization
		return nil
	}))
	subscriptionCalls := 0
	w.stripe.respond = func(_ string, path string, _ map[string]string, _ string) (any, error) {
		switch path {
		case "/v1/subscriptions":
			subscriptionCalls++
			if subscriptionCalls == 1 {
				return map[string]any{"data": []any{}}, nil
			}
			return map[string]any{"data": []any{w.stripe.current}}, nil
		case "/v1/checkout/sessions/cs_completed":
			return map[string]any{"id": "cs_completed", "status": "complete"}, nil
		default:
			return nil, fmt.Errorf("unexpected request %s", path)
		}
	}
	_, err := w.service.Checkout(t.Context(), w.organization, CheckoutRequest{PriceID: w.price.ID})
	require.ErrorIs(t, err, ErrConflict)
	assert.Equal(t, 2, subscriptionCalls)
	state, err := w.store.Snapshot()
	require.NoError(t, err)
	assert.Equal(t, "sub_expected", state.Organizations[w.organization].SubscriptionID)
	assert.Equal(t, w.paidTemplate, state.Organizations[w.organization].TemplateID)
	for _, request := range w.stripe.requests {
		assert.False(t, request.method == "post" && request.path == "/v1/checkout/sessions")
	}
}

func TestWebhookDuplicateIsDurableAndDoesNotRepeatMigration(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	body, signature := signedEvent(
		t,
		w.config.WebhookSecret,
		"evt_duplicate",
		"customer.subscription.updated",
		w.stripe.current,
	)
	require.NoError(t, w.service.HandleWebhook(t.Context(), body, signature))
	require.NoError(t, w.service.HandleWebhook(t.Context(), body, signature))
	state, err := w.store.Snapshot()
	require.NoError(t, err)
	require.Len(t, state.Events, 1)
	onDisk, err := readStoredState(w.store.path)
	require.NoError(t, err)
	require.Contains(t, onDisk.Events, "evt_duplicate")
	require.NoError(t, w.service.ProcessPending(t.Context()))
	require.NoError(t, w.service.HandleWebhook(t.Context(), body, signature))
	require.NoError(t, w.service.ProcessPending(t.Context()))
	require.Len(t, w.anchor.applications, 1)
	assert.Equal(
		t,
		templateApplication{organizationID: w.organization, templateID: w.paidTemplate},
		w.anchor.applications[0],
	)
	state, err = w.store.Snapshot()
	require.NoError(t, err)
	assert.Equal(t, 1, state.Events["evt_duplicate"].Attempts)
}

func TestWebhookUsesCurrentSubscriptionInsteadOfOutOfOrderPayload(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	stale := subscriptionPayload("sub_expected", "cus_expected", "canceled", "price_paid")
	body, signature := signedEvent(t, w.config.WebhookSecret, "evt_stale", "customer.subscription.deleted", stale)
	require.NoError(t, w.service.HandleWebhook(t.Context(), body, signature))
	require.NoError(t, w.service.ProcessPending(t.Context()))
	state, err := w.store.Snapshot()
	require.NoError(t, err)
	assert.Equal(t, "active", state.Organizations[w.organization].Status)
	assert.Equal(t, w.paidTemplate, state.Organizations[w.organization].TemplateID)
	require.Len(t, w.anchor.applications, 1)
	assert.Equal(t, w.paidTemplate, w.anchor.applications[0].templateID)
}

func TestWebhookRejectsBadSignatureWithoutPersistingWork(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	body, signature := signedEvent(
		t,
		"whsec_other",
		"evt_bad_signature",
		"customer.subscription.updated",
		w.stripe.current,
	)
	require.Error(t, w.service.HandleWebhook(t.Context(), body, signature))
	state, err := w.store.Snapshot()
	require.NoError(t, err)
	assert.Empty(t, state.Events)
	assert.Empty(t, w.anchor.applications)
}

func TestFailedLicenseMigrationRemainsDurableAndRetries(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	w.anchor.applyError = errors.New("Anchor license migration unavailable")
	body, signature := signedEvent(
		t,
		w.config.WebhookSecret,
		"evt_retry",
		"customer.subscription.updated",
		w.stripe.current,
	)
	require.NoError(t, w.service.HandleWebhook(t.Context(), body, signature))
	require.NoError(t, w.service.ProcessPending(t.Context()))
	state, err := w.store.Snapshot()
	require.NoError(t, err)
	assert.Equal(t, w.freeTemplate, state.Organizations[w.organization].TemplateID)
	assert.NotEmpty(t, state.Organizations[w.organization].SyncError)
	assert.NotEmpty(t, state.Events["evt_retry"].LastError)
	onDisk, err := readStoredState(w.store.path)
	require.NoError(t, err)
	assert.NotEmpty(t, onDisk.Events["evt_retry"].LastError)
	w.anchor.applyError = nil
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		event := state.Events["evt_retry"]
		event.NextAttempt = time.Time{}
		state.Events["evt_retry"] = event
		return nil
	}))
	statePath := w.store.path
	require.NoError(t, w.store.Close())
	w.store, err = OpenStore(statePath, w.config.ExpectedAccountID, w.config.ProductID, w.freeTemplate)
	require.NoError(t, err)
	w.service, err = NewService(w.config, w.stripe, w.anchor, w.store)
	require.NoError(t, err)
	require.NoError(t, w.service.ProcessPending(t.Context()))
	state, err = w.store.Snapshot()
	require.NoError(t, err)
	assert.Equal(t, w.paidTemplate, state.Organizations[w.organization].TemplateID)
	assert.Empty(t, state.Organizations[w.organization].SyncError)
	require.Len(t, w.anchor.applications, 2)
}

func TestWebhookRejectsLiveAndForeignAccountEvents(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		field string
		value any
	}{
		{name: "live event", field: "livemode", value: true},
		{name: "foreign account", field: "account", value: "acct_foreign"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			w := newServiceWorld(t)
			body, _ := signedEvent(
				t,
				w.config.WebhookSecret,
				"evt_refused",
				"customer.subscription.updated",
				w.stripe.current,
			)
			var envelope map[string]any
			require.NoError(t, json.Unmarshal(body, &envelope))
			envelope[test.field] = test.value
			body, err := json.Marshal(envelope)
			require.NoError(t, err)
			signature := signEventBody(t, w.config.WebhookSecret, int64(envelope["created"].(float64)), body)
			require.Error(t, w.service.HandleWebhook(t.Context(), body, signature))
			state, err := w.store.Snapshot()
			require.NoError(t, err)
			assert.Empty(t, state.Events)
			assert.Empty(t, w.anchor.applications)
		})
	}
}

func TestForeignWebhookCustomerCannotSelectOrganizationWithMetadata(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	foreign := subscriptionPayload("sub_foreign", "cus_foreign", "active", "price_paid")
	foreign["metadata"] = map[string]string{"anchor_organization_id": w.organization,
		"anchor_product_id": w.config.ProductID}
	body, signature := signedEvent(
		t,
		w.config.WebhookSecret,
		"evt_foreign_customer",
		"customer.subscription.updated",
		foreign,
	)
	require.NoError(t, w.service.HandleWebhook(t.Context(), body, signature))
	require.NoError(t, w.service.ProcessPending(t.Context()))
	state, err := w.store.Snapshot()
	require.NoError(t, err)
	assert.Equal(t, "ignored", state.Events["evt_foreign_customer"].Status)
	assert.Empty(t, state.Events["evt_foreign_customer"].OrganizationID)
	assert.Empty(t, w.stripe.requests)
	assert.Empty(t, w.anchor.applications)
}

func TestSyncRefusesSubscriptionFromAnotherCustomer(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	w.stripe.current["customer"] = "cus_foreign"
	_, err := w.service.SyncOrganization(t.Context(), w.organization)
	require.Error(t, err)
	assert.Empty(t, w.anchor.applications)
	state, err := w.store.Snapshot()
	require.NoError(t, err)
	assert.Equal(t, "cus_expected", state.Organizations[w.organization].CustomerID)
	assert.Equal(t, w.freeTemplate, state.Organizations[w.organization].TemplateID)
}

func TestPendingUpgradeDoesNotGrantUnpaidPlan(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	oldPrice := w.price
	oldPrice.ID = ksuid.New().String()
	oldPrice.StripePriceID = "price_current"
	oldPrice.TemplateID = w.freeTemplate
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		state.Prices[oldPrice.ID] = oldPrice
		return nil
	}))
	w.setSubscription("active", oldPrice.StripePriceID)
	w.stripe.current["pending_update"] = map[string]any{"expires_at": time.Now().Add(time.Hour).Unix()}
	organization, err := w.service.ChangeSubscription(
		t.Context(),
		w.organization,
		SubscriptionRequest{PriceID: w.price.ID},
	)
	require.NoError(t, err)
	assert.True(t, organization.PendingUpdate)
	assert.Equal(t, w.freeTemplate, organization.TemplateID)
	assert.Equal(t, oldPrice.ID, organization.PriceID)
	for _, application := range w.anchor.applications {
		assert.Equal(t, w.freeTemplate, application.templateID)
	}
	w.anchor.applications = nil
	w.setSubscription("active", w.price.StripePriceID)
	organization, err = w.service.SyncOrganization(t.Context(), w.organization)
	require.NoError(t, err)
	assert.False(t, organization.PendingUpdate)
	assert.Equal(t, w.paidTemplate, organization.TemplateID)
	require.Len(t, w.anchor.applications, 1)
}

func TestCanceledSubscriptionUsesFallbackAndKeepsAdjustedValues(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	w.anchor.snapshot.Organizations[0].TemplateID = w.paidTemplate
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		organization := state.Organizations[w.organization]
		organization.TemplateID = w.paidTemplate
		state.Organizations[w.organization] = organization
		return nil
	}))
	w.stripe.current["status"] = "canceled"
	organization, err := w.service.SyncOrganization(t.Context(), w.organization)
	require.NoError(t, err)
	assert.Equal(t, w.freeTemplate, organization.TemplateID)
	assert.InDelta(t, 7, organization.LicenseValues["seat_limit"], 0.001)
	require.Len(t, w.anchor.applications, 1)
	assert.Equal(t, w.freeTemplate, w.anchor.applications[0].templateID)
}

func TestFirstObservedCanceledSubscriptionUsesFallback(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	w.anchor.snapshot.Organizations[0].TemplateID = w.paidTemplate
	w.setSubscription("canceled", "price_paid")
	require.NoError(t, w.store.Update(func(state *StoredState) error {
		organization := state.Organizations[w.organization]
		organization.SubscriptionID = ""
		organization.TemplateID = w.paidTemplate
		state.Organizations[w.organization] = organization
		return nil
	}))
	organization, err := w.service.SyncOrganization(t.Context(), w.organization)
	require.NoError(t, err)
	assert.Equal(t, "sub_expected", organization.SubscriptionID)
	assert.Equal(t, "canceled", organization.Status)
	assert.Equal(t, w.freeTemplate, organization.TemplateID)
	assert.InDelta(t, 7, organization.LicenseValues["seat_limit"], 0.001)
	require.Len(t, w.anchor.applications, 1)
	assert.Equal(t, w.freeTemplate, w.anchor.applications[0].templateID)
}

func TestScheduledCancellationKeepsPaidLicenseUntilSubscriptionEnds(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	w.stripe.current["cancel_at_period_end"] = true
	organization, err := w.service.SyncOrganization(t.Context(), w.organization)
	require.NoError(t, err)
	assert.True(t, organization.CancelAtPeriodEnd)
	assert.Equal(t, "active", organization.Status)
	assert.Equal(t, w.paidTemplate, organization.TemplateID)
	require.Len(t, w.anchor.applications, 1)
	assert.Equal(t, w.paidTemplate, w.anchor.applications[0].templateID)
}

func TestSyncRejectsOrganizationOutsideAnchorProduct(t *testing.T) {
	t.Parallel()
	w := newServiceWorld(t)
	_, err := w.service.SyncOrganization(t.Context(), "org_"+ksuid.New().String())
	require.Error(t, err)
	assert.Empty(t, w.stripe.requests)
	assert.Empty(t, w.anchor.applications)
}

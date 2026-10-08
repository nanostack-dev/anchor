package billing_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v87"

	"anchor/internal/stripebilling/billing"
)

type httpRecord struct {
	method  string
	target  *url.URL
	headers http.Header
	body    string
}

type httpFixture struct {
	status   int
	body     string
	location string
	err      error
}

type stripeTransport struct {
	fixtures []httpFixture
	records  []httpRecord
}

func (s *stripeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	var body []byte
	if request.Body != nil {
		var err error
		body, err = io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
	}
	s.records = append(s.records, httpRecord{method: request.Method, target: request.URL,
		headers: request.Header.Clone(), body: string(body)})
	if len(s.fixtures) == 0 {
		return nil, errors.New("unexpected Stripe HTTP call")
	}
	fixture := s.fixtures[0]
	s.fixtures = s.fixtures[1:]
	if fixture.err != nil {
		return nil, fixture.err
	}
	headers := http.Header{}
	if fixture.location != "" {
		headers.Set("Location", fixture.location)
	}
	return &http.Response{StatusCode: fixture.status, Header: headers,
		Body: io.NopCloser(strings.NewReader(fixture.body)), Request: request}, nil
}

func sandboxIdentity() httpFixture {
	return httpFixture{status: http.StatusOK,
		body: `{"id":"acct_expected","settings":{"dashboard":{"display_name":"Test sandbox"}}}`}
}

func nativeConfig() billing.ConnectionConfig {
	return billing.ConnectionConfig{AccountID: "acct_expected", AuthMethod: "API_KEY",
		APIKey: "sk_test_fixture", WebhookSecret: "whsec_fixture"}
}

func TestSDKGatewayPinsIdentityAndEncodesFormParameters(t *testing.T) {
	t.Parallel()
	transport := &stripeTransport{fixtures: []httpFixture{sandboxIdentity(),
		{status: http.StatusOK, body: `{"id":"cus_encoded","livemode":false}`}}}
	gateway, err := billing.NewSDKGateway(nativeConfig(), &http.Client{Transport: transport})
	require.NoError(t, err)
	name := "Organization & status=live ; $(echo untrusted)"
	result, err := gateway.Client().V1Customers.Create(t.Context(), &stripe.CustomerCreateParams{
		IdempotencyKey: stripe.String("intent_expected"),
		Name:           stripe.String(name),
		Metadata:       map[string]string{"anchor_product_id": "prd_scope"},
	})
	require.NoError(t, err)
	assert.Equal(t, "cus_encoded", result.ID)
	require.Len(t, transport.records, 2)
	for _, request := range transport.records {
		assert.Equal(t, "https", request.target.Scheme)
		assert.Equal(t, "api.stripe.com", request.target.Host)
		assert.Equal(t, "Bearer sk_test_fixture", request.headers.Get("Authorization"))
		assert.Equal(t, "acct_expected", request.headers.Get("Stripe-Account"))
		assert.Equal(t, billing.StripeVersion, request.headers.Get("Stripe-Version"))
	}
	request := transport.records[1]
	assert.Equal(t, "/v1/customers", request.target.Path)
	assert.Empty(t, request.target.RawQuery)
	assert.Equal(t, "intent_expected", request.headers.Get("Idempotency-Key"))
	assert.Equal(t, "application/x-www-form-urlencoded", request.headers.Get("Content-Type"))
	params, err := url.ParseQuery(request.body)
	require.NoError(t, err)
	assert.Equal(t, name, params.Get("name"))
	assert.Equal(t, "prd_scope", params.Get("metadata[anchor_product_id]"))
	assert.Empty(t, params.Get("status"))
}

func TestSDKRefundPinsAccountAndPersistsRequestIdentity(t *testing.T) {
	t.Parallel()
	transport := &stripeTransport{fixtures: []httpFixture{
		sandboxIdentity(),
		{
			status: http.StatusOK,
			body:   `{"id":"re_expected","charge":"ch_expected","amount":1500,"currency":"usd","status":"pending"}`,
		},
	}}
	gateway, err := billing.NewSDKGateway(nativeConfig(), &http.Client{Transport: transport})
	require.NoError(t, err)
	refund, err := gateway.Client().V1Refunds.Create(t.Context(), &stripe.RefundCreateParams{
		IdempotencyKey: new("saved-charge-intent"), Charge: new("ch_expected"), Amount: new(int64(1500)),
		Metadata: map[string]string{"anchor_refund_action_id": "action_expected"},
	})
	require.NoError(t, err)
	require.Equal(t, stripe.RefundStatusPending, refund.Status)
	require.Len(t, transport.records, 2)
	request := transport.records[1]
	require.Equal(t, "acct_expected", request.headers.Get("Stripe-Account"))
	require.Equal(t, "saved-charge-intent", request.headers.Get("Idempotency-Key"))
	params, err := url.ParseQuery(request.body)
	require.NoError(t, err)
	require.Equal(t, "1500", params.Get("amount"))
	require.Equal(t, "ch_expected", params.Get("charge"))
	require.Equal(t, "action_expected", params.Get("metadata[anchor_refund_action_id]"))
	require.Empty(t, params.Get("reason"))
}

func TestSDKGatewayEncodesQueriesWithoutChangingScope(t *testing.T) {
	t.Parallel()
	transport := &stripeTransport{fixtures: []httpFixture{sandboxIdentity(),
		{status: http.StatusOK, body: `{"object":"list","data":[]}`}}}
	gateway, err := billing.NewSDKGateway(nativeConfig(), &http.Client{Transport: transport})
	require.NoError(t, err)
	customer := "cus_expected&customer=cus_foreign"
	require.NoError(
		t,
		gateway.Client().
			V1Subscriptions.List(t.Context(), &stripe.SubscriptionListParams{Customer: stripe.String(customer), Status: stripe.String("all")}).
			Err(),
	)
	require.Len(t, transport.records, 2)
	request := transport.records[1]
	assert.Equal(t, customer, request.target.Query().Get("customer"))
	assert.Equal(t, []string{customer}, request.target.Query()["customer"])
	assert.Equal(t, "all", request.target.Query().Get("status"))
	assert.Empty(t, request.body)
}

func TestSDKGatewayRefusesForeignAccountBeforeMutation(t *testing.T) {
	t.Parallel()
	transport := &stripeTransport{fixtures: []httpFixture{
		{status: http.StatusOK, body: `{"id":"acct_foreign"}`},
	}}
	gateway, err := billing.NewSDKGateway(nativeConfig(), &http.Client{Transport: transport})
	require.NoError(t, err)
	_, err = gateway.Client().V1Customers.Create(t.Context(), nil)
	require.ErrorContains(t, err, "another account")
	require.Len(t, transport.records, 1)
	assert.Equal(t, http.MethodGet, transport.records[0].method)
	assert.Equal(t, "/v1/account", transport.records[0].target.Path)
}

func TestSDKGatewayRefusesLiveCredentialsAndInvalidRequests(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"sk_live_refused", "rk_live_refused", "pk_test_refused"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			cfg := nativeConfig()
			cfg.APIKey = key
			_, err := billing.NewSDKGateway(cfg, nil)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), key)
		})
	}
}

func TestSDKGatewayRejectsLiveAndErrorResponsesWithoutLeakingCredentials(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		fixture httpFixture
		message string
	}{
		{"live object", httpFixture{status: http.StatusOK, body: `{"id":"cus_live","livemode":true}`}, "connection"},
		{
			"live list entry",
			httpFixture{status: http.StatusOK, body: `{"data":[{"id":"sub_live","livemode":true}]}`},
			"connection",
		},
		{
			"API error even with success status",
			httpFixture{
				status: http.StatusOK,
				body:   `{"error":{"message":"sk_test_fixture rk_live_fixture whsec_fixture were refused"}}`,
			},
			"[redacted]",
		},
		{"invalid JSON", httpFixture{status: http.StatusBadGateway, body: "not JSON"}, "connection"},
		{"transport error", httpFixture{err: errors.New("sk_test_fixture is private")}, "connection"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			transport := &stripeTransport{fixtures: []httpFixture{sandboxIdentity(), test.fixture}}
			gateway, err := billing.NewSDKGateway(nativeConfig(), &http.Client{Transport: transport})
			require.NoError(t, err)
			_, err = gateway.Client().V1Customers.Retrieve(t.Context(), "cus_expected", nil)
			require.ErrorContains(t, err, test.message)
			assert.NotContains(t, err.Error(), nativeConfig().APIKey)
			assert.NotContains(t, err.Error(), nativeConfig().WebhookSecret)
			assert.NotContains(t, err.Error(), "rk_live_fixture")
		})
	}
}

func TestSDKGatewayNeverFollowsRedirectsWithCredentials(t *testing.T) {
	t.Parallel()
	transport := &stripeTransport{fixtures: []httpFixture{{status: http.StatusFound,
		location: "https://foreign.example/capture", body: "redirect"}}}
	gateway, err := billing.NewSDKGateway(nativeConfig(), &http.Client{Transport: transport})
	require.NoError(t, err)
	_, err = gateway.Account(t.Context())
	require.ErrorContains(t, err, "connection")
	require.Len(t, transport.records, 1)
	assert.Equal(t, "api.stripe.com", transport.records[0].target.Host)
}

func TestSDKGatewayRespectsCallerCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	gateway, err := billing.NewSDKGateway(nativeConfig(), &http.Client{Transport: contextTransport{}})
	require.NoError(t, err)
	_, err = gateway.Account(ctx)
	require.Error(t, err)
}

type contextTransport struct{}

func (contextTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return nil, request.Context().Err()
}

func TestSDKGatewaysKeepCredentialsAndAccountsIsolated(t *testing.T) {
	t.Parallel()
	for _, accountID := range []string{"acct_first", "acct_second"} {
		t.Run(accountID, func(t *testing.T) {
			t.Parallel()
			config := nativeConfig()
			config.AccountID = accountID
			config.APIKey = "rk_test_fixture"
			transport := &stripeTransport{fixtures: []httpFixture{
				{status: http.StatusOK, body: `{"id":"` + accountID + `"}`},
				{status: http.StatusOK, body: `{"id":"cus_scoped","livemode":false}`},
			}}
			gateway, err := billing.NewSDKGateway(config, &http.Client{Transport: transport})
			require.NoError(t, err)
			params := &stripe.CustomerCreateParams{StripeAccount: stripe.String("acct_foreign"),
				StripeContext: stripe.String("acct_foreign"),
				Headers: http.Header{
					"Authorization":  []string{"Bearer sk_live_fixture"},
					"Stripe-Version": []string{"old-version"},
				}}
			_, err = gateway.Client().V1Customers.Create(t.Context(), params)
			require.NoError(t, err)
			require.Len(t, transport.records, 2)
			for _, request := range transport.records {
				assert.Equal(t, "Bearer rk_test_fixture", request.headers.Get("Authorization"))
				assert.Equal(t, accountID, request.headers.Get("Stripe-Account"))
				assert.Equal(t, stripe.APIVersion, request.headers.Get("Stripe-Version"))
				assert.Empty(t, request.headers.Get("Stripe-Context"))
			}
		})
	}
}

type closingBody struct {
	io.Reader
	closed bool
}

func (b *closingBody) Close() error { b.closed = true; return nil }

type closingTransport struct{ bodies []*closingBody }

func (t *closingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	payload := `{"id":"cus_body","livemode":false}`
	if request.URL.Path == "/v1/account" {
		payload = `{"id":"acct_expected"}`
	}
	body := &closingBody{Reader: strings.NewReader(payload)}
	t.bodies = append(t.bodies, body)
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: body, Request: request}, nil
}

func TestSDKGatewayClosesOriginalBodiesBeforeDecodingTypedResults(t *testing.T) {
	t.Parallel()
	transport := &closingTransport{}
	gateway, err := billing.NewSDKGateway(nativeConfig(), &http.Client{Transport: transport})
	require.NoError(t, err)
	customer, err := gateway.Client().V1Customers.Retrieve(t.Context(), "cus_body", nil)
	require.NoError(t, err)
	assert.Equal(t, "cus_body", customer.ID)
	require.Len(t, transport.bodies, 2)
	for _, body := range transport.bodies {
		assert.True(t, body.closed)
	}
}

func TestSDKGatewayRefusesNullListResourcesBeforeSDKIteration(t *testing.T) {
	t.Parallel()
	transport := &stripeTransport{fixtures: []httpFixture{sandboxIdentity(),
		{status: http.StatusOK, body: `{"object":"list","data":[null],"has_more":false}`},
	}}
	gateway, err := billing.NewSDKGateway(nativeConfig(), &http.Client{Transport: transport})
	require.NoError(t, err)
	list := gateway.Client().V1Subscriptions.List(t.Context(), nil)
	require.Error(t, list.Err())
	assert.Empty(t, list.Data())
}

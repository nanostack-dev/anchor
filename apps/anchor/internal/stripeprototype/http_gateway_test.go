package stripeprototype_test

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

	"anchor/internal/stripeprototype"
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

func nativeConfig() stripeprototype.ConnectionConfig {
	return stripeprototype.ConnectionConfig{AccountID: "acct_expected", AuthMethod: "API_KEY",
		APIKey: "sk_test_gatewayFixture", WebhookSecret: "whsec_gatewayFixture"}
}

func TestHTTPGatewayPinsIdentityAndEncodesFormParameters(t *testing.T) {
	t.Parallel()
	transport := &stripeTransport{fixtures: []httpFixture{sandboxIdentity(),
		{status: http.StatusOK, body: `{"id":"cus_encoded","livemode":false}`}}}
	gateway, err := stripeprototype.NewHTTPGateway(nativeConfig(), &http.Client{Transport: transport})
	require.NoError(t, err)
	name := "Organization & status=live ; $(echo untrusted)"
	var result struct {
		ID string `json:"id"`
	}
	require.NoError(t, gateway.Request(t.Context(), "post", "/v1/customers",
		map[string]string{"name": name, "metadata[anchor_product_id]": "prd_scope"}, "intent_expected", &result))
	assert.Equal(t, "cus_encoded", result.ID)
	require.Len(t, transport.records, 2)
	for _, request := range transport.records {
		assert.Equal(t, "https", request.target.Scheme)
		assert.Equal(t, "api.stripe.com", request.target.Host)
		assert.Equal(t, "Bearer sk_test_gatewayFixture", request.headers.Get("Authorization"))
		assert.Equal(t, "acct_expected", request.headers.Get("Stripe-Account"))
		assert.Equal(t, stripeprototype.StripeVersion, request.headers.Get("Stripe-Version"))
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

func TestHTTPGatewayEncodesQueriesWithoutChangingScope(t *testing.T) {
	t.Parallel()
	transport := &stripeTransport{fixtures: []httpFixture{sandboxIdentity(),
		{status: http.StatusOK, body: `{"object":"list","data":[]}`}}}
	gateway, err := stripeprototype.NewHTTPGateway(nativeConfig(), &http.Client{Transport: transport})
	require.NoError(t, err)
	customer := "cus_expected&customer=cus_foreign"
	require.NoError(t, gateway.Request(t.Context(), "get", "/v1/subscriptions",
		map[string]string{"customer": customer, "status": "all"}, "", nil))
	require.Len(t, transport.records, 2)
	request := transport.records[1]
	assert.Equal(t, customer, request.target.Query().Get("customer"))
	assert.Equal(t, []string{customer}, request.target.Query()["customer"])
	assert.Equal(t, "all", request.target.Query().Get("status"))
	assert.Empty(t, request.body)
}

func TestHTTPGatewayRefusesForeignAccountBeforeMutation(t *testing.T) {
	t.Parallel()
	transport := &stripeTransport{fixtures: []httpFixture{
		{status: http.StatusOK, body: `{"id":"acct_foreign"}`},
	}}
	gateway, err := stripeprototype.NewHTTPGateway(nativeConfig(), &http.Client{Transport: transport})
	require.NoError(t, err)
	err = gateway.Request(t.Context(), "post", "/v1/customers", nil, "", nil)
	require.ErrorContains(t, err, "another account")
	require.Len(t, transport.records, 1)
	assert.Equal(t, http.MethodGet, transport.records[0].method)
	assert.Equal(t, "/v1/account", transport.records[0].target.Path)
}

func TestHTTPGatewayRefusesLiveCredentialsAndInvalidRequests(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"sk_live_refused", "rk_live_refused", "pk_test_refused"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			cfg := nativeConfig()
			cfg.APIKey = key
			_, err := stripeprototype.NewHTTPGateway(cfg, nil)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), key)
		})
	}
	for _, path := range []string{"https://foreign.example/v1/customers", "/v2/accounts", "/v1/customers?account=foreign"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			transport := &stripeTransport{}
			gateway, err := stripeprototype.NewHTTPGateway(nativeConfig(), &http.Client{Transport: transport})
			require.NoError(t, err)
			require.Error(t, gateway.Request(t.Context(), "post", path, nil, "", nil))
			assert.Empty(t, transport.records)
		})
	}
}

func TestHTTPGatewayRejectsLiveAndErrorResponsesWithoutLeakingCredentials(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		fixture httpFixture
		message string
	}{
		{"live object", httpFixture{status: http.StatusOK, body: `{"id":"cus_live","livemode":true}`}, "live object"},
		{
			"live list entry",
			httpFixture{status: http.StatusOK, body: `{"data":[{"id":"sub_live","livemode":true}]}`},
			"live object",
		},
		{
			"API error even with success status",
			httpFixture{
				status: http.StatusOK,
				body:   `{"error":{"message":"sk_test_gatewayFixture rk_live_otherSecret whsec_gatewayFixture were refused"}}`,
			},
			"[redacted]",
		},
		{"invalid JSON", httpFixture{status: http.StatusBadGateway, body: "not JSON"}, "invalid"},
		{"transport error", httpFixture{err: errors.New("sk_test_gatewayFixture is private")}, "connection"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			transport := &stripeTransport{fixtures: []httpFixture{sandboxIdentity(), test.fixture}}
			gateway, err := stripeprototype.NewHTTPGateway(nativeConfig(), &http.Client{Transport: transport})
			require.NoError(t, err)
			err = gateway.Request(t.Context(), "get", "/v1/customers/cus_expected", nil, "", nil)
			require.ErrorContains(t, err, test.message)
			assert.NotContains(t, err.Error(), nativeConfig().APIKey)
			assert.NotContains(t, err.Error(), nativeConfig().WebhookSecret)
			assert.NotContains(t, err.Error(), "rk_live_otherSecret")
		})
	}
}

func TestHTTPGatewayNeverFollowsRedirectsWithCredentials(t *testing.T) {
	t.Parallel()
	transport := &stripeTransport{fixtures: []httpFixture{{status: http.StatusFound,
		location: "https://foreign.example/capture", body: "redirect"}}}
	gateway, err := stripeprototype.NewHTTPGateway(nativeConfig(), &http.Client{Transport: transport})
	require.NoError(t, err)
	_, err = gateway.Account(t.Context())
	require.ErrorContains(t, err, "redirects are refused")
	require.Len(t, transport.records, 1)
	assert.Equal(t, "api.stripe.com", transport.records[0].target.Host)
}

func TestHTTPGatewayRespectsCallerCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	gateway, err := stripeprototype.NewHTTPGateway(nativeConfig(), &http.Client{Transport: contextTransport{}})
	require.NoError(t, err)
	_, err = gateway.Account(ctx)
	require.Error(t, err)
}

type contextTransport struct{}

func (contextTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return nil, request.Context().Err()
}

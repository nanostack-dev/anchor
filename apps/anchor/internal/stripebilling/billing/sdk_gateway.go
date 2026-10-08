package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/stripe/stripe-go/v87"
)

const (
	stripeAPIOrigin            = "https://api.stripe.com"
	maximumStripeResponseBytes = 8 << 20
)

type sdkGateway struct {
	config ConnectionConfig
	client *stripe.Client
}

func NewSDKGateway(config ConnectionConfig, client *http.Client) (StripeGateway, error) {
	if !strings.HasPrefix(config.AccountID, "acct_") || len(config.AccountID) <= len("acct_") ||
		!validAnchorID(config.AccountID) {
		return nil, errors.New("stripe sandbox account is required")
	}
	if (config.AuthMethod != "" && config.AuthMethod != "API_KEY") ||
		(!strings.HasPrefix(config.APIKey, "sk_test_") && !strings.HasPrefix(config.APIKey, "rk_test_")) ||
		strings.IndexFunc(
			config.APIKey,
			func(character rune) bool { return character <= ' ' || character >= 127 },
		) != -1 {
		return nil, errors.New("stripe SDK authentication requires a sandbox test API key; live keys are refused")
	}
	boundedClient := &http.Client{}
	if client != nil {
		*boundedClient = *client
	}
	if boundedClient.Timeout == 0 || boundedClient.Timeout > apiTimeout {
		boundedClient.Timeout = apiTimeout
	}
	boundedClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	transport := boundedClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	boundedClient.Transport = sandboxTransport{transport: transport, key: config.APIKey, accountID: config.AccountID}
	remote := stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{
		HTTPClient: boundedClient, MaxNetworkRetries: new(int64(0)), URL: stripe.String(stripeAPIOrigin),
		LeveledLogger: &stripe.LeveledLogger{Level: stripe.LevelNull}, EnableTelemetry: new(false),
	})
	gateway := &sdkGateway{config: config}
	backend := &guardedBackend{backend: remote, accountID: config.AccountID, identity: gateway.Account,
		secrets: []string{config.APIKey, config.WebhookSecret}}
	gateway.client = stripe.NewClient(
		config.APIKey,
		stripe.WithBackends(&stripe.Backends{API: backend, Connect: backend, Uploads: backend}),
	)
	return gateway, nil
}

func (g *sdkGateway) Client() *stripe.Client { return g.client }

func (g *sdkGateway) Account(ctx context.Context) (Account, error) {
	ctx, cancel := context.WithTimeout(ctx, identityTimeout)
	defer cancel()
	remote, err := g.client.V1Accounts.Retrieve(ctx, nil)
	if err != nil {
		return Account{}, err
	}
	if remote.ID != g.config.AccountID {
		return Account{}, errors.New("stripe returned another account; sandbox request refused")
	}
	name := remote.ID
	if remote.Settings != nil && remote.Settings.Dashboard != nil && remote.Settings.Dashboard.DisplayName != "" {
		name = remote.Settings.Dashboard.DisplayName
	} else if remote.BusinessProfile != nil && remote.BusinessProfile.Name != "" {
		name = remote.BusinessProfile.Name
	}
	return Account{ID: remote.ID, Name: name, Mode: Sandbox}, nil
}

type unsupportedBackend struct{}

func (unsupportedBackend) CallStreaming(
	string,
	string,
	string,
	stripe.ParamsContainer,
	stripe.StreamingLastResponseSetter,
) error {
	return errors.New("stripe streaming operations are unsupported by billing")
}

func (unsupportedBackend) CallMultipart(
	string,
	string,
	string,
	string,
	*bytes.Buffer,
	*stripe.Params,
	stripe.LastResponseSetter,
) error {
	return errors.New("stripe multipart operations are unsupported by billing")
}
func (unsupportedBackend) SetMaxNetworkRetries(int64) {}

type guardedBackend struct {
	unsupportedBackend
	backend   stripe.Backend
	accountID string
	identity  func(context.Context) (Account, error)
	secrets   []string
}

func (b *guardedBackend) prepare(method, path string, params *stripe.Params) error {
	if (method != http.MethodGet && method != http.MethodPost && method != http.MethodDelete) ||
		!strings.HasPrefix(path, "/v1/") || strings.ContainsAny(path, "?#\r\n") {
		return errors.New("invalid Stripe SDK request")
	}
	if params.Context == nil {
		return errors.New("stripe SDK request context is required")
	}
	if err := params.Context.Err(); err != nil {
		return err
	}
	if path != "/v1/account" {
		if _, err := b.identity(params.Context); err != nil {
			return err
		}
	}
	params.StripeAccount = stripe.String(b.accountID)
	params.StripeContext = nil
	params.Headers = nil
	return nil
}

func (b *guardedBackend) Call(
	method, path, key string,
	params stripe.ParamsContainer,
	result stripe.LastResponseSetter,
) error {
	if err := b.prepare(method, path, params.GetParams()); err != nil {
		return err
	}
	if err := b.safeError(b.backend.Call(method, path, key, params, result)); err != nil {
		return err
	}
	return validateSDKResource(result)
}

func (b *guardedBackend) CallRaw(
	method, path, key string,
	body []byte,
	params *stripe.Params,
	result stripe.LastResponseSetter,
) error {
	if err := b.prepare(method, path, params); err != nil {
		return err
	}
	return b.safeError(b.backend.CallRaw(method, path, key, body, params, result))
}

func (b *guardedBackend) safeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	var stripeError *stripe.Error
	if !errors.As(err, &stripeError) {
		return errors.New("stripe sandbox request failed; check the integration connection")
	}
	message := stripeError.Msg
	for _, secret := range b.secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
	}
	message = regexp.MustCompile(`\b(?:oak_|whsec_|(?:sk|rk|pk)_(?:test|live)_)[A-Za-z0-9_]+\b`).
		ReplaceAllString(message, "[redacted]")
	return fmt.Errorf("stripe sandbox: %s", message)
}

type sandboxTransport struct {
	transport http.RoundTripper
	key       string
	accountID string
}

func (t sandboxTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme != "https" || request.URL.Host != "api.stripe.com" ||
		!strings.HasPrefix(request.URL.Path, "/v1/") {
		return nil, errors.New("stripe SDK target origin is refused")
	}
	request.Header.Set("Authorization", "Bearer "+t.key)
	request.Header.Set("Stripe-Account", t.accountID)
	request.Header.Set("Stripe-Version", StripeVersion)
	request.Header.Del("Stripe-Context")
	response, err := t.transport.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	originalBody := response.Body
	defer func() { _ = originalBody.Close() }()
	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
		return nil, errors.New("stripe redirects are refused")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maximumStripeResponseBytes+1))
	if err != nil || len(data) > maximumStripeResponseBytes {
		return nil, errors.New("stripe response exceeds the permitted size or could not be read")
	}
	if err = refuseLivePayload(data); err != nil {
		return nil, err
	}
	response.Body = io.NopCloser(bytes.NewReader(data))
	return response, nil
}

func refuseLivePayload(data []byte) error {
	var envelope struct {
		Error *stripe.Error `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return errors.New("stripe returned an invalid API response")
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return errors.New("stripe returned an invalid API response")
	}
	if containsNullListResource(payload) {
		return errors.New("stripe returned a missing list resource")
	}
	if containsLiveObject(payload) {
		return errors.New("stripe returned a live object; sandbox integration refused it")
	}
	return nil
}

func containsLiveObject(value any) bool {
	switch nested := value.(type) {
	case map[string]any:
		if live, ok := nested["livemode"].(bool); ok && live {
			return true
		}
		for _, child := range nested {
			if containsLiveObject(child) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(nested, containsLiveObject)
	}
	return false
}

func validateSDKResource(result stripe.LastResponseSetter) error {
	var id string
	switch resource := result.(type) {
	case *stripe.Account:
		id = resource.ID
	case *stripe.Customer:
		id = resource.ID
	case *stripe.Product:
		id = resource.ID
	case *stripe.Price:
		id = resource.ID
	case *stripe.Subscription:
		id = resource.ID
	case *stripe.Charge:
		id = resource.ID
	case *stripe.Invoice:
		id = resource.ID
	case *stripe.RadarEarlyFraudWarning:
		id = resource.ID
	case *stripe.Refund:
		id = resource.ID
	case *stripe.CheckoutSession:
		id = resource.ID
	case *stripe.BillingPortalConfiguration:
		id = resource.ID
	case *stripe.BillingPortalSession:
		id = resource.ID
	default:
		return nil
	}
	if id == "" {
		return errors.New("stripe returned an SDK resource without an identity")
	}
	return nil
}

func containsNullListResource(value any) bool {
	switch nested := value.(type) {
	case map[string]any:
		if data, ok := nested["data"].([]any); ok {
			for _, child := range data {
				if child == nil {
					return true
				}
			}
		}
		for _, child := range nested {
			if containsNullListResource(child) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(nested, containsNullListResource)
	}
	return false
}

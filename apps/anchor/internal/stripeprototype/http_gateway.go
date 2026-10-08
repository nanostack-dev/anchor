package stripeprototype

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

const (
	stripeAPIOrigin            = "https://api.stripe.com"
	maximumStripeResponseBytes = 8 << 20
)

type httpGateway struct {
	config ConnectionConfig
	client *http.Client
}

func NewHTTPGateway(config ConnectionConfig, client *http.Client) (StripeGateway, error) {
	if !strings.HasPrefix(config.AccountID, "acct_") || len(config.AccountID) <= len("acct_") ||
		!validAnchorID(config.AccountID) {
		return nil, errors.New("stripe sandbox account is required")
	}
	if (config.AuthMethod != "" && config.AuthMethod != "API_KEY") ||
		(!strings.HasPrefix(config.APIKey, "sk_test_") && !strings.HasPrefix(config.APIKey, "rk_test_")) ||
		strings.IndexFunc(config.APIKey, func(character rune) bool {
			return character <= ' ' || character >= 127
		}) != -1 {
		return nil, errors.New("stripe HTTP authentication requires a sandbox test API key; live keys are refused")
	}
	boundedClient := &http.Client{}
	if client != nil {
		*boundedClient = *client
	}
	if boundedClient.Timeout == 0 || boundedClient.Timeout > apiTimeout {
		boundedClient.Timeout = apiTimeout
	}
	boundedClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &httpGateway{config: config, client: boundedClient}, nil
}

func (g *httpGateway) Account(ctx context.Context) (Account, error) {
	ctx, cancel := context.WithTimeout(ctx, identityTimeout)
	defer cancel()
	var remote struct {
		ID              string `json:"id"`
		BusinessProfile struct {
			Name string `json:"name"`
		} `json:"business_profile"`
		Settings struct {
			Dashboard struct {
				DisplayName string `json:"display_name"`
			} `json:"dashboard"`
		} `json:"settings"`
	}
	if err := g.request(ctx, http.MethodGet, "/v1/account", nil, "", &remote); err != nil {
		return Account{}, err
	}
	if remote.ID != g.config.AccountID {
		return Account{}, errors.New("stripe returned another account; sandbox request refused")
	}
	name := remote.Settings.Dashboard.DisplayName
	if name == "" {
		name = remote.BusinessProfile.Name
	}
	if name == "" {
		name = remote.ID
	}
	return Account{ID: remote.ID, Name: name, Mode: Sandbox}, nil
}

func (g *httpGateway) Request(
	ctx context.Context, method, path string, params map[string]string, idempotency string, result any,
) error {
	if (method != "get" && method != "post" && method != "delete") || !strings.HasPrefix(path, "/v1/") ||
		strings.ContainsAny(path, "?#\r\n") {
		return errors.New("invalid Stripe API request")
	}
	if _, err := g.Account(ctx); err != nil {
		return err
	}
	return g.request(ctx, strings.ToUpper(method), path, params, idempotency, result)
}

func (g *httpGateway) request(
	ctx context.Context, method, path string, params map[string]string, idempotency string, result any,
) error {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	target := stripeAPIOrigin + path
	var body io.Reader
	if method == http.MethodGet {
		if len(values) > 0 {
			target += "?" + values.Encode()
		}
	} else {
		body = strings.NewReader(values.Encode())
	}
	request, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return errors.New("stripe sandbox request could not be prepared")
	}
	request.Header.Set("Authorization", "Bearer "+g.config.APIKey)
	request.Header.Set("Stripe-Account", g.config.AccountID)
	request.Header.Set("Stripe-Version", StripeVersion)
	if method != http.MethodGet {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if idempotency != "" {
		request.Header.Set("Idempotency-Key", idempotency)
	}
	response, err := g.client.Do(request)
	if err != nil {
		return errors.New("stripe sandbox request failed; check the integration connection")
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(response.Body, maximumStripeResponseBytes+1))
	if err != nil || len(data) > maximumStripeResponseBytes {
		return errors.New("stripe sandbox response could not be read")
	}
	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
		return errors.New("stripe redirects are refused")
	}
	if response.StatusCode == http.StatusNoContent && result == nil && len(data) == 0 {
		return nil
	}
	return g.decodeResponse(response.StatusCode, data, result)
}

func (g *httpGateway) decodeResponse(status int, data []byte, result any) error {
	var envelope struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return errors.New("stripe returned an invalid sandbox API response")
	}
	if envelope.Error != nil {
		message := envelope.Error.Message
		for _, secret := range []string{g.config.APIKey, g.config.WebhookSecret} {
			if secret != "" {
				message = strings.ReplaceAll(message, secret, "[redacted]")
			}
		}
		message = regexp.MustCompile(`\b(?:whsec_|(?:sk|rk|pk)_(?:test|live)_)[A-Za-z0-9_]+\b`).
			ReplaceAllString(message, "[redacted]")
		return fmt.Errorf("stripe sandbox: %s", message)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("stripe sandbox request was refused with HTTP %d", status)
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return errors.New("stripe returned an invalid sandbox API response")
	}
	if containsLiveObject(payload) {
		return errors.New("stripe returned a live object; sandbox integration refused it")
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(data, result); err != nil {
		return errors.New("stripe returned an incompatible sandbox API response")
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

//nolint:testpackage // Tests inspect encrypted provider storage and replace private transport seams.
package stripe

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stripeSDK "github.com/stripe/stripe-go/v87"

	"anchor/internal/domain/integration"
	"anchor/internal/security/encryption"
	serviceconfig "anchor/internal/service/config"
	"anchor/internal/stripebilling/billing"
)

const (
	testAccount       = "acct_Test123"
	testAPIKey        = "sk_test_fixtureOnly"
	testWebhookSecret = "whsec_fixtureOnly"
)

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index + 1)
	}
	encryptionService, err := encryption.NewService(&serviceconfig.CoreConfig{
		Encryption: serviceconfig.EncryptionConfig{
			GlobalKey: base64.StdEncoding.EncodeToString(key), GlobalKeyVersion: "v1",
		},
	})
	require.NoError(t, err)
	return NewProvider(NewProviderParams{EncryptionService: encryptionService, Logger: zerolog.Nop()})
}

func configuredInstance(t *testing.T, p *Provider, cfg Config) integration.Instance {
	t.Helper()
	body, err := json.Marshal(cfg)
	require.NoError(t, err)
	stored, err := p.PrepareConfigForStorage(t.Context(), body)
	require.NoError(t, err)
	return integration.Instance{ProviderType: integration.ProviderTypeStripe, ConfigJSON: stored}
}

func testConfig() Config {
	return Config{AccountID: testAccount, APIKey: testAPIKey, WebhookSecret: testWebhookSecret,
		ReturnURL: "https://anchor.example.test/products/prd_example/integrations/stripe"}
}

func TestCredentialsAreEncryptedBeforeStorageAndResolvedOnlyForBilling(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t)
	instance := configuredInstance(t, p, testConfig())
	assert.NotContains(t, string(instance.ConfigJSON), testAPIKey)
	assert.NotContains(t, string(instance.ConfigJSON), testWebhookSecret)
	var stored Config
	require.NoError(t, json.Unmarshal(instance.ConfigJSON, &stored))
	assert.True(t, secrets.IsVersionedEncryptedSecret(stored.APIKey))
	assert.True(t, secrets.IsVersionedEncryptedSecret(stored.WebhookSecret))
	assert.Equal(t, AuthMethodAPIKey, stored.AuthMethod)
	require.NoError(t, p.ValidateConfig(t.Context(), instance.ConfigJSON))
	resolved, err := p.ResolveBillingConfig(t.Context(), instance)
	require.NoError(t, err)
	assert.Equal(t, testAPIKey, resolved.APIKey)
	assert.Equal(t, testWebhookSecret, resolved.WebhookSecret)
	assert.Equal(t, testAccount, resolved.AccountID)
	assert.Equal(t, string(AuthMethodAPIKey), resolved.AuthMethod)
}

func TestBlankUpdatePreservesEncryptedCredentialsAndAuthenticationMethod(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t)
	cfg := testConfig()
	cfg.AuthMethod, cfg.APIKey, cfg.ReturnURL = AuthMethodLocalCLI, "", "http://127.0.0.1:3307/products/prd_example/integrations/stripe"
	instance := configuredInstance(t, p, cfg)
	updated, err := p.PrepareUpdatedConfigForStorage(t.Context(), instance.ConfigJSON,
		[]byte(`{"api_key":"","webhook_secret":"","return_url":"http://127.0.0.1:3307/updated"}`))
	require.NoError(t, err)
	var before, after Config
	require.NoError(t, json.Unmarshal(instance.ConfigJSON, &before))
	require.NoError(t, json.Unmarshal(updated, &after))
	assert.Equal(t, before.WebhookSecret, after.WebhookSecret)
	assert.Equal(t, before.AccountID, after.AccountID)
	assert.Equal(t, AuthMethodLocalCLI, after.AuthMethod)
	assert.Equal(t, "http://127.0.0.1:3307/updated", after.ReturnURL)
	require.NoError(t, p.ValidateConfig(t.Context(), updated))
}

func TestReplacingCredentialsEncryptsNewValues(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t)
	instance := configuredInstance(t, p, testConfig())
	updated, err := p.PrepareUpdatedConfigForStorage(t.Context(), instance.ConfigJSON,
		[]byte(`{"api_key":"rk_test_replacementFixture","webhook_secret":"whsec_replacementFixture"}`))
	require.NoError(t, err)
	assert.NotContains(t, string(updated), "rk_test_replacementFixture")
	assert.NotContains(t, string(updated), "whsec_replacementFixture")
	instance.ConfigJSON = updated
	resolved, err := p.ResolveBillingConfig(t.Context(), instance)
	require.NoError(t, err)
	assert.Equal(t, "rk_test_replacementFixture", resolved.APIKey)
	assert.Equal(t, "whsec_replacementFixture", resolved.WebhookSecret)
}

func TestInvalidStripeConfigurationIsRefused(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		change func(*Config)
	}{
		{"live key", func(cfg *Config) { cfg.APIKey = "sk_live_refusedFixture" }},
		{"live restricted key", func(cfg *Config) { cfg.APIKey = "rk_live_refusedFixture" }},
		{"publishable key", func(cfg *Config) { cfg.APIKey = "pk_test_refusedFixture" }},
		{"invalid account", func(cfg *Config) { cfg.AccountID = "acct_test/injected" }},
		{"invalid signing secret", func(cfg *Config) { cfg.WebhookSecret = "not-a-signing-secret" }},
		{"nonlocal HTTP", func(cfg *Config) { cfg.ReturnURL = "http://anchor.example.test/billing" }},
		{
			"return URL credentials",
			func(cfg *Config) { cfg.ReturnURL = "https://user:password@anchor.example.test/billing" },
		},
		{
			"query in return URL",
			func(cfg *Config) { cfg.ReturnURL = "https://anchor.example.test/billing?next=elsewhere" },
		},
		{"unknown authentication", func(cfg *Config) { cfg.AuthMethod = "UNKNOWN" }},
		{"remote local CLI", func(cfg *Config) { cfg.AuthMethod = AuthMethodLocalCLI }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			p := newTestProvider(t)
			cfg := testConfig()
			test.change(&cfg)
			instance := configuredInstance(t, p, cfg)
			err := p.ValidateConfig(t.Context(), instance.ConfigJSON)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), cfg.APIKey)
			assert.NotContains(t, err.Error(), cfg.WebhookSecret)
		})
	}
}

func TestUnsafeCredentialStorageFailsClosed(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t)
	body, err := json.Marshal(testConfig())
	require.NoError(t, err)
	_, err = p.ResolveBillingConfig(t.Context(), integration.Instance{ConfigJSON: body})
	require.ErrorContains(t, err, "not safely encrypted")
	withoutCipher := NewProvider(NewProviderParams{Logger: zerolog.Nop()})
	_, err = withoutCipher.PrepareConfigForStorage(t.Context(), body)
	require.ErrorContains(t, err, "encryption is unavailable")
	assert.NotContains(t, err.Error(), testAPIKey)
}

func TestConnectionVerificationPinsSandboxAccountAndHidesRemoteErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		status    int
		body      string
		wantError bool
	}{
		{"matching sandbox", http.StatusOK, `{"id":"acct_Test123","object":"account"}`, false},
		{"another account", http.StatusOK, `{"id":"acct_Another123","object":"account"}`, true},
		{
			"remote authentication error",
			http.StatusUnauthorized,
			`{"error":{"type":"invalid_request_error","message":"sk_test_fixtureOnly must never reach the UI"}}`,
			true,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/account", r.URL.Path)
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, testAccount, r.Header.Get("Stripe-Account"))
				assert.Equal(t, billing.StripeVersion, r.Header.Get("Stripe-Version"))
				assert.Contains(t, r.Header.Get("Authorization"), testAPIKey)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				_, err := fmt.Fprint(w, test.body)
				assert.NoError(t, err)
			}))
			t.Cleanup(server.Close)
			p := newTestProvider(t)
			p.backends = stripeSDK.NewBackendsWithConfig(&stripeSDK.BackendConfig{
				URL: stripeSDK.String(server.URL), HTTPClient: server.Client(), MaxNetworkRetries: stripeSDK.Int64(0),
				LeveledLogger: &stripeSDK.LeveledLogger{Level: stripeSDK.LevelNull},
			})
			instance := configuredInstance(t, p, testConfig())
			err := p.VerifyConnection(t.Context(), &instance)
			if test.wantError {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), testAPIKey)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type identityRunner struct {
	identity string
	commands [][]string
}

func (r *identityRunner) Run(_ context.Context, args []string) ([]byte, error) {
	r.commands = append(r.commands, args)
	return []byte(r.identity), nil
}

func TestLocalCLIConnectionUsesExistingLoginWithoutAnAPIKey(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"test", "live"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			p := newTestProvider(t)
			runner := &identityRunner{identity: fmt.Sprintf(`{"account_id":"%s","mode":"%s"}`, testAccount, mode)}
			p.cliRunner = runner
			cfg := testConfig()
			cfg.AuthMethod, cfg.APIKey, cfg.ReturnURL = AuthMethodLocalCLI, "", "http://127.0.0.1:3307/products/prd_example/integrations/stripe"
			instance := configuredInstance(t, p, cfg)
			err := p.VerifyConnection(t.Context(), &instance)
			if mode == "test" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "live mode is refused")
			}
			require.Len(t, runner.commands, 1)
			assert.Equal(t, []string{"whoami", "--format", "json", "--color", "off"}, runner.commands[0])
		})
	}
}

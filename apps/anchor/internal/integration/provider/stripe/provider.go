package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"
	stripeSDK "github.com/stripe/stripe-go/v87"
	"go.uber.org/fx"

	"anchor/internal/domain/integration"
	"anchor/internal/integration/provider"
	"anchor/internal/security/encryption"
	"anchor/internal/stripeprototype"
)

var (
	_ provider.ConfigUpdateStorageProvider = (*Provider)(nil)
	_ provider.ConnectionVerifier          = (*Provider)(nil)
)

const (
	configVersion  = int32(1)
	cipherContext  = "stripe-billing-credentials"
	requestTimeout = 30 * time.Second
)

type Provider struct {
	cipher    *secrets.VersionedCipher
	cipherErr error
	backends  *stripeSDK.Backends
	cliRunner stripeprototype.CommandRunner
}

type NewProviderParams struct {
	fx.In
	EncryptionService *encryption.Service
	Logger            zerolog.Logger
}

func NewProvider(params NewProviderParams) *Provider {
	p := &Provider{backends: stripeSDK.NewBackendsWithConfig(&stripeSDK.BackendConfig{
		HTTPClient: &http.Client{
			Timeout:       requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		MaxNetworkRetries: stripeSDK.Int64(0),
		LeveledLogger:     &stripeSDK.LeveledLogger{Level: stripeSDK.LevelNull},
	})}
	if params.EncryptionService == nil {
		p.cipherErr = errors.New("stripe credentials require a configured encryption key")
	} else {
		p.cipher, p.cipherErr = params.EncryptionService.NewCipher(cipherContext)
	}
	if p.cipherErr != nil {
		params.Logger.Error().Msg("Stripe provider credential encryption is unavailable")
	}
	return p
}

func (*Provider) Type() string               { return string(integration.ProviderTypeStripe) }
func (*Provider) LatestConfigVersion() int32 { return configVersion }

func (p *Provider) ValidateConfig(_ context.Context, configJSON []byte) error {
	cfg, err := p.resolveConfig(configJSON)
	if err != nil {
		return err
	}
	return validateConfig(cfg)
}

func (*Provider) MigrateConfig(_ context.Context, _ int32, configJSON []byte) ([]byte, error) {
	return configJSON, nil
}

func (p *Provider) PrepareConfigForStorage(_ context.Context, configJSON []byte) ([]byte, error) {
	cfg, err := parseConfig(configJSON)
	if err != nil {
		return nil, err
	}
	if cfg.AuthMethod == "" {
		cfg.AuthMethod = AuthMethodAPIKey
	}
	if cfg.APIKey, err = p.encryptSecret(cfg.APIKey); err != nil {
		return nil, err
	}
	if cfg.WebhookSecret, err = p.encryptSecret(cfg.WebhookSecret); err != nil {
		return nil, err
	}
	return json.Marshal(cfg) // #nosec G117 -- Credential fields are encrypted before serialization.
}

func (p *Provider) PrepareUpdatedConfigForStorage(
	ctx context.Context, currentConfigJSON, configJSON []byte,
) ([]byte, error) {
	current, err := parseConfig(currentConfigJSON)
	if err != nil {
		return nil, err
	}
	updated, err := parseConfig(configJSON)
	if err != nil {
		return nil, err
	}
	if updated.APIKey == "" {
		updated.APIKey = current.APIKey
	}
	if updated.WebhookSecret == "" {
		updated.WebhookSecret = current.WebhookSecret
	}
	if updated.AccountID == "" {
		updated.AccountID = current.AccountID
	}
	if updated.AuthMethod == "" {
		updated.AuthMethod = current.AuthMethod
	}
	if updated.ReturnURL == "" {
		updated.ReturnURL = current.ReturnURL
	}
	body, err := json.Marshal(updated) // #nosec G117 -- Existing encrypted credentials are preserved for normalization.
	if err != nil {
		return nil, errors.New("invalid Stripe configuration")
	}
	return p.PrepareConfigForStorage(ctx, body)
}

func (p *Provider) VerifyConnection(ctx context.Context, instance *integration.Instance) error {
	cfg, err := p.ResolveBillingConfig(ctx, *instance)
	if err != nil {
		return err
	}
	if cfg.AuthMethod == string(AuthMethodLocalCLI) {
		cli, cliErr := stripeprototype.NewCLI(cfg.AccountID, p.cliRunner)
		if cliErr != nil {
			return errors.New("stripe CLI sandbox account is invalid")
		}
		_, cliErr = cli.Account(ctx)
		return cliErr
	}
	client := stripeSDK.NewClient(cfg.APIKey, stripeSDK.WithBackends(p.backends))
	params := &stripeSDK.AccountRetrieveParams{}
	params.SetStripeAccount(cfg.AccountID)
	account, err := client.V1Accounts.Retrieve(ctx, params)
	if err != nil {
		return errors.New("stripe sandbox connection failed; check the account and test API key")
	}
	if account.ID != cfg.AccountID {
		return errors.New("stripe returned another account; sandbox connection refused")
	}
	return nil
}

func (p *Provider) ResolveBillingConfig(
	_ context.Context, instance integration.Instance,
) (stripeprototype.ConnectionConfig, error) {
	cfg, err := p.resolveConfig(instance.ConfigJSON)
	if err != nil {
		return stripeprototype.ConnectionConfig{}, err
	}
	if err = validateConfig(cfg); err != nil {
		return stripeprototype.ConnectionConfig{}, err
	}
	if cfg.AccountID == "" || cfg.ReturnURL == "" || cfg.WebhookSecret == "" ||
		(cfg.AuthMethod == AuthMethodAPIKey && cfg.APIKey == "") {
		return stripeprototype.ConnectionConfig{}, errors.New(
			"configure the Stripe sandbox account, authentication, webhook secret and return URL",
		)
	}
	return stripeprototype.ConnectionConfig{
		AuthMethod: string(cfg.AuthMethod),
		APIKey:     cfg.APIKey, AccountID: cfg.AccountID, WebhookSecret: cfg.WebhookSecret, ReturnURL: cfg.ReturnURL,
	}, nil
}

func parseConfig(body []byte) (Config, error) {
	var cfg Config
	if len(body) != 0 {
		if err := json.Unmarshal(body, &cfg); err != nil {
			return Config{}, errors.New("invalid Stripe configuration")
		}
	}
	cfg.AccountID = strings.TrimSpace(cfg.AccountID)
	cfg.AuthMethod = AuthMethod(strings.TrimSpace(string(cfg.AuthMethod)))
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.WebhookSecret = strings.TrimSpace(cfg.WebhookSecret)
	cfg.ReturnURL = strings.TrimSpace(cfg.ReturnURL)
	return cfg, nil
}

func (p *Provider) encryptSecret(value string) (string, error) {
	if value == "" || secrets.IsVersionedEncryptedSecret(value) {
		return value, nil
	}
	if p.cipher == nil || p.cipherErr != nil {
		return "", errors.New("stripe credential encryption is unavailable")
	}
	encrypted, err := p.cipher.EncryptString(value)
	if err != nil {
		return "", errors.New("stripe credential encryption failed")
	}
	return encrypted, nil
}

func (p *Provider) resolveConfig(body []byte) (Config, error) {
	cfg, err := parseConfig(body)
	if err != nil {
		return Config{}, err
	}
	if cfg.AuthMethod == "" {
		cfg.AuthMethod = AuthMethodAPIKey
	}
	if cfg.APIKey, err = p.decryptSecret(cfg.APIKey); err != nil {
		return Config{}, err
	}
	if cfg.WebhookSecret, err = p.decryptSecret(cfg.WebhookSecret); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (p *Provider) decryptSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !secrets.IsVersionedEncryptedSecret(value) || p.cipher == nil || p.cipherErr != nil {
		return "", errors.New("stripe stored credentials are not safely encrypted")
	}
	decrypted, err := p.cipher.DecryptString(value)
	if err != nil {
		return "", errors.New("stripe credential decryption failed")
	}
	return decrypted, nil
}

func validateConfig(cfg Config) error {
	if err := validate.ValidateStruct(cfg); err != nil {
		return errors.New("invalid Stripe account or return URL")
	}
	if cfg.AuthMethod != AuthMethodAPIKey && cfg.AuthMethod != AuthMethodLocalCLI {
		return errors.New("stripe authentication must use a test API key or the local CLI")
	}
	if cfg.AccountID != "" && (len(cfg.AccountID) <= len("acct_") || strings.IndexFunc(
		strings.TrimPrefix(cfg.AccountID, "acct_"), func(character rune) bool {
			valid := character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
			return !valid
		},
	) != -1) {
		return errors.New("invalid Stripe sandbox account identifier")
	}
	if cfg.APIKey != "" && !strings.HasPrefix(cfg.APIKey, "sk_test_") && !strings.HasPrefix(cfg.APIKey, "rk_test_") {
		return errors.New("stripe requires a sandbox test API key; live keys are refused")
	}
	if cfg.WebhookSecret != "" && !strings.HasPrefix(cfg.WebhookSecret, "whsec_") {
		return errors.New("stripe requires a webhook signing secret")
	}
	return validateReturnURL(cfg)
}

func validateReturnURL(cfg Config) error {
	if cfg.ReturnURL == "" {
		return nil
	}
	parsed, err := url.Parse(cfg.ReturnURL)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		parsed.Hostname() == "" {
		return errors.New("stripe return URL must be an HTTPS origin or a local HTTP URL")
	}
	address := net.ParseIP(parsed.Hostname())
	local := strings.EqualFold(parsed.Hostname(), "localhost") || address != nil && address.IsLoopback()
	if cfg.AuthMethod == AuthMethodLocalCLI && (parsed.Scheme != "http" || !local) {
		return errors.New("stripe local CLI authentication requires a loopback HTTP return URL")
	}
	if parsed.Scheme != "https" && (parsed.Scheme != "http" || !local) {
		return errors.New("stripe return URL requires HTTPS outside local development")
	}
	return nil
}

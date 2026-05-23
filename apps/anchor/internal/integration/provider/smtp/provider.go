package smtp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"anchor/internal/domain/integration"
	"anchor/internal/integration/provider"
	"anchor/internal/security/encryption"

	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

var (
	_ provider.Provider                    = (*Provider)(nil)
	_ provider.ConfigStorageProvider       = (*Provider)(nil)
	_ provider.ConfigUpdateStorageProvider = (*Provider)(nil)
	_ provider.ConnectionVerifier          = (*Provider)(nil)
	_ provider.Mailer                      = (*Provider)(nil)
)

const (
	smtpProviderType        = "SMTP"
	smtpLatestConfigVer     = int32(1)
	smtpConfigCipherContext = "smtp-credentials"

	defaultPort = 587
)

var errMissingEncryptionKey = errors.New("global encryption key is not configured")

// Provider implements an SMTP-based outbound email integration. It is
// deliberately generic so it works with any provider speaking SMTP-AUTH over
// STARTTLS or implicit TLS (SendGrid, Mailgun, Proton, AWS SES, self-hosted
// relays, ...).
type Provider struct {
	configCipher    *secrets.VersionedCipher
	configCipherErr error
	logger          zerolog.Logger
}

type NewProviderParams struct {
	fx.In

	EncryptionService *encryption.Service
	Logger            zerolog.Logger
}

func NewProvider(p NewProviderParams) *Provider {
	var (
		cipher *secrets.VersionedCipher
		err    error
	)
	if p.EncryptionService == nil {
		err = errMissingEncryptionKey
	} else {
		cipher, err = p.EncryptionService.NewCipher(smtpConfigCipherContext)
	}
	componentLogger := p.Logger.With().Str("component", "smtp_provider").Logger()
	if err != nil {
		componentLogger.Error().Err(err).Msg("failed to initialize smtp config cipher")
	}

	return &Provider{
		configCipher:    cipher,
		configCipherErr: err,
		logger:          componentLogger,
	}
}

func (p *Provider) Type() string               { return smtpProviderType }
func (p *Provider) LatestConfigVersion() int32 { return smtpLatestConfigVer }

func (p *Provider) ValidateConfig(_ context.Context, configJSON []byte) error {
	if len(configJSON) == 0 || string(configJSON) == "{}" {
		return nil
	}

	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return fmt.Errorf("invalid smtp config: %w", err)
	}

	if strings.TrimSpace(cfg.Host) == "" {
		return errors.New("smtp config: host is required")
	}
	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("smtp config: port %d is out of range", cfg.Port)
	}
	if strings.TrimSpace(cfg.Username) == "" {
		return errors.New("smtp config: username is required")
	}
	if strings.TrimSpace(cfg.FromAddress) == "" {
		return errors.New("smtp config: from_address is required")
	}

	switch cfg.Encryption {
	case "", EncryptionStartTLS, EncryptionTLS, EncryptionNone:
	default:
		return fmt.Errorf("smtp config: unknown encryption mode %q", cfg.Encryption)
	}
	switch cfg.AuthMethod {
	case "", AuthMethodPlain, AuthMethodLogin:
	default:
		return fmt.Errorf("smtp config: unknown auth method %q", cfg.AuthMethod)
	}

	return nil
}

func (p *Provider) MigrateConfig(_ context.Context, fromVersion int32, configJSON []byte) ([]byte, error) {
	if fromVersion >= smtpLatestConfigVer {
		return configJSON, nil
	}
	return configJSON, nil
}

// PrepareConfigForStorage encrypts the SMTP password before persistence.
// All other fields are stored in plaintext to keep operator UX (host/port/
// username) editable from the admin UI without ceremony.
func (p *Provider) PrepareConfigForStorage(_ context.Context, configJSON []byte) ([]byte, error) {
	if len(configJSON) == 0 || string(configJSON) == "{}" {
		return configJSON, nil
	}

	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("invalid smtp config: %w", err)
	}

	password := strings.TrimSpace(cfg.Password)
	if password == "" || secrets.IsVersionedEncryptedSecret(password) {
		return configJSON, nil
	}

	if p.configCipherErr != nil {
		return nil, fmt.Errorf("failed to initialize smtp config cipher: %w", p.configCipherErr)
	}
	if p.configCipher == nil {
		return nil, errMissingEncryptionKey
	}

	encrypted, err := p.configCipher.EncryptString(password)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt smtp password: %w", err)
	}
	cfg.Password = encrypted

	normalized, err := json.Marshal(cfg) // #nosec G117 -- password is encrypted before marshaling
	if err != nil {
		return nil, fmt.Errorf("failed to marshal smtp config: %w", err)
	}
	return normalized, nil
}

// PrepareUpdatedConfigForStorage preserves the existing encrypted password when
// the update payload leaves password blank, matching the write-only API
// contract for SMTP credentials.
func (p *Provider) PrepareUpdatedConfigForStorage(
	ctx context.Context,
	currentConfigJSON []byte,
	configJSON []byte,
) ([]byte, error) {
	if len(configJSON) == 0 || string(configJSON) == "{}" {
		return configJSON, nil
	}

	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("invalid smtp config: %w", err)
	}
	if strings.TrimSpace(cfg.Password) != "" {
		return p.PrepareConfigForStorage(ctx, configJSON)
	}
	if len(currentConfigJSON) == 0 || string(currentConfigJSON) == "{}" {
		return p.PrepareConfigForStorage(ctx, configJSON)
	}

	var currentCfg Config
	if err := json.Unmarshal(currentConfigJSON, &currentCfg); err != nil {
		return nil, fmt.Errorf("invalid existing smtp config: %w", err)
	}

	cfg.Password = currentCfg.Password
	// #nosec G117 -- password is already encrypted or preserved from stored config.
	mergedConfigJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal smtp config: %w", err)
	}

	return p.PrepareConfigForStorage(ctx, mergedConfigJSON)
}

// resolveConfig decrypts persisted secrets and returns the runtime config.
func (p *Provider) resolveConfig(configJSON []byte) (Config, error) {
	if len(configJSON) == 0 || string(configJSON) == "{}" {
		return Config{}, errors.New("smtp config is empty")
	}

	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return Config{}, fmt.Errorf("invalid smtp config: %w", err)
	}

	password := strings.TrimSpace(cfg.Password)
	if password == "" {
		return cfg, nil
	}
	if !secrets.IsVersionedEncryptedSecret(password) {
		return cfg, nil
	}

	if p.configCipherErr != nil {
		return Config{}, fmt.Errorf("failed to initialize smtp config cipher: %w", p.configCipherErr)
	}
	if p.configCipher == nil {
		return Config{}, errMissingEncryptionKey
	}

	decrypted, err := p.configCipher.DecryptString(password)
	if err != nil {
		return Config{}, fmt.Errorf("failed to decrypt smtp password: %w", err)
	}
	cfg.Password = decrypted

	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	if cfg.Encryption == "" {
		cfg.Encryption = EncryptionStartTLS
	}
	if cfg.AuthMethod == "" {
		cfg.AuthMethod = AuthMethodPlain
	}

	return cfg, nil
}

// VerifyConnection performs an EHLO + AUTH probe against the configured SMTP
// server using the persisted credentials. On success the email service flips
// the integration lifecycle state to ACTIVE.
func (p *Provider) VerifyConnection(ctx context.Context, instance *integration.Instance) error {
	cfg, err := p.resolveConfig(instance.ConfigJSON)
	if err != nil {
		return err
	}
	client, err := dialSMTP(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	if err = authenticate(client, cfg); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	return client.Quit()
}

// Send transmits a fully rendered OutboundMessage through the configured SMTP
// transport. The email service is responsible for rendering, idempotency,
// and retry; this method handles the network conversation and returns the
// on-the-wire Message-ID for audit-trail correlation.
func (p *Provider) Send(
	ctx context.Context,
	instance *integration.Instance,
	msg provider.OutboundMessage,
) error {
	cfg, err := p.resolveConfig(instance.ConfigJSON)
	if err != nil {
		return err
	}
	client, err := dialSMTP(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	if err = authenticate(client, cfg); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if _, err = sendMessage(client, cfg, msg); err != nil {
		return err
	}
	return client.Quit()
}

// SenderIdentity returns the From address + display name configured on the
// SMTP instance. Decrypts the persisted config so the email service can
// stamp the audit-trail record without re-implementing the schema.
func (p *Provider) SenderIdentity(_ context.Context, instance *integration.Instance) (provider.Address, error) {
	cfg, err := p.resolveConfig(instance.ConfigJSON)
	if err != nil {
		return provider.Address{}, err
	}
	return provider.Address{Email: cfg.FromAddress, Name: cfg.FromName}, nil
}

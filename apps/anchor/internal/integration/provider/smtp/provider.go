package smtp

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"anchor/internal/domain/integration"
	"anchor/internal/integration/provider"
	"anchor/internal/security/encryption"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
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
	logger          *slog.Logger
}

func NewProvider(encryptionService *encryption.Service, logger *slog.Logger) *Provider {
	var (
		cipher *secrets.VersionedCipher
		err    error
	)
	if encryptionService == nil {
		err = errMissingEncryptionKey
	} else {
		cipher, err = encryptionService.NewCipher(smtpConfigCipherContext)
	}
	componentLogger := logger.With(slog.String("component", "smtp_provider"))
	if err != nil {
		componentLogger.Error("failed to initialize smtp config cipher", log.Err(err))
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

	// The stored password authenticates to the stored server only; a new destination needs it sent again.
	effectivePort := func(port int) int { return cmp.Or(port, defaultPort) }
	if !strings.EqualFold(strings.TrimSpace(cfg.Host), strings.TrimSpace(currentCfg.Host)) ||
		effectivePort(cfg.Port) != effectivePort(currentCfg.Port) {
		return nil, fault.BadRequest("SMTP_PASSWORD_REQUIRED",
			"Send the SMTP password again when the host or the port changes.")
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
	// last_error is returned to the tenant, so it names the step, never the peer's reply or dial outcome.
	client, err := dialSMTP(ctx, cfg)
	if err != nil {
		p.logger.DebugContext(ctx, "smtp verification could not connect",
			log.Err(err),
			slog.String("instance_id", instance.ID),
		)
		return errors.New("smtp: could not connect to the server")
	}
	defer func() { _ = client.Close() }()
	if err = authenticate(client, cfg); err != nil {
		p.logger.DebugContext(ctx, "smtp verification was refused",
			log.Err(err),
			slog.String("instance_id", instance.ID),
		)
		return errors.New("smtp: the server refused the credentials")
	}
	if err = client.Quit(); err != nil {
		p.logger.DebugContext(ctx, "smtp verification did not close cleanly",
			log.Err(err),
			slog.String("instance_id", instance.ID),
		)
		return errors.New("smtp: the server ended the session with an error")
	}
	return nil
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

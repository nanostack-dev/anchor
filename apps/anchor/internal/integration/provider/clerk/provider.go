package clerk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"anchor/internal/domain/integration"
	"anchor/internal/events"
	"anchor/internal/integration/provider"
	"anchor/internal/repository"
	"anchor/internal/security/encryption"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/rs/zerolog"
	svix "github.com/svix/svix-webhooks/go"
	"go.uber.org/fx"
)

var _ provider.Provider = (*Provider)(nil)
var _ provider.ConfigStorageProvider = (*Provider)(nil)
var _ provider.WebhookEventProvider = (*Provider)(nil)

var errMissingEncryptionKey = errors.New("global encryption key is not configured")

const (
	clerkProviderType        = "CLERK"
	clerkLatestConfigVer     = int32(1)
	svixIDHeader             = "svix-id"
	svixTimestampHeader      = "svix-timestamp"
	svixSignatureHeader      = "svix-signature"
	clerkUsersPageSize       = 500
	clerkDefaultBaseURL      = "https://api.clerk.com/v1"
	clerkConfigCipherContext = "clerk-api-key"

	// HTTP transport timeouts for the Clerk API client.
	clerkHTTPDialTimeout           = 10 * time.Second
	clerkHTTPIdleConnTimeout       = 90 * time.Second
	clerkHTTPTLSTimeout            = 10 * time.Second
	clerkHTTPRequestTimeout        = 30 * time.Second
	clerkHTTPExpectContinueTimeout = 1 * time.Second
	clerkHTTPMaxIdleConns          = 100
)

// Provider implements the provider.Provider interface for Clerk identity webhooks.
type Provider struct {
	productUserRepo repository.ProductUserRepository
	auditLogRepo    repository.IntegrationAuditLogRepository
	configCipher    *secrets.VersionedCipher
	configCipherErr error
	httpClient      *http.Client
	baseURL         string
	events          events.Emitter
	transactor      transactor.Transactor
	logger          zerolog.Logger
}

type NewProviderParams struct {
	fx.In

	ProductUserRepo   repository.ProductUserRepository
	AuditLogRepo      repository.IntegrationAuditLogRepository
	EncryptionService *encryption.Service
	Events            events.Emitter
	Transactor        transactor.Transactor
	Logger            zerolog.Logger
}

// NewProvider creates a new Clerk provider.
func NewProvider(p NewProviderParams) *Provider {
	var (
		cipher *secrets.VersionedCipher
		err    error
	)
	if p.EncryptionService == nil {
		err = errMissingEncryptionKey
	} else {
		cipher, err = p.EncryptionService.NewCipher(clerkConfigCipherContext)
	}
	componentLogger := p.Logger.With().Str("component", "clerk_provider").Logger()
	if err != nil {
		componentLogger.Error().Err(err).Msg("failed to initialize clerk config cipher")
	}

	return &Provider{
		productUserRepo: p.ProductUserRepo,
		auditLogRepo:    p.AuditLogRepo,
		configCipher:    cipher,
		configCipherErr: err,
		events:          p.Events,
		transactor:      p.Transactor,
		httpClient: &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout: clerkHTTPDialTimeout,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          clerkHTTPMaxIdleConns,
			IdleConnTimeout:       clerkHTTPIdleConnTimeout,
			TLSHandshakeTimeout:   clerkHTTPTLSTimeout,
			ExpectContinueTimeout: clerkHTTPExpectContinueTimeout,
		}, Timeout: clerkHTTPRequestTimeout},
		baseURL: clerkDefaultBaseURL,
		logger:  componentLogger,
	}
}

func (p *Provider) Type() string {
	return clerkProviderType
}

func (p *Provider) LatestConfigVersion() int32 {
	return clerkLatestConfigVer
}

// Config represents the config stored in integration_instances.config_json.
type Config struct {
	// Clerk API key for backfill/reconciliation calls (future use).
	APIKey string `json:"api_key,omitempty"`
	// Optional operator note for known integration issues.
	IssueReason *string `json:"issue_reason,omitempty"`
}

func (p *Provider) ValidateConfig(_ context.Context, configJSON []byte) error {
	if len(configJSON) == 0 || string(configJSON) == "{}" {
		return nil // empty config is valid for v1
	}
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return fmt.Errorf("invalid clerk config: %w", err)
	}
	return nil
}

func (p *Provider) PrepareConfigForStorage(_ context.Context, configJSON []byte) ([]byte, error) {
	if len(configJSON) == 0 || string(configJSON) == "{}" {
		return configJSON, nil
	}

	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("invalid clerk config: %w", err)
	}

	if strings.TrimSpace(cfg.APIKey) == "" {
		return configJSON, nil
	}
	if secrets.IsVersionedEncryptedSecret(strings.TrimSpace(cfg.APIKey)) {
		return configJSON, nil
	}
	if p.configCipherErr != nil {
		return nil, fmt.Errorf("failed to initialize clerk config cipher: %w", p.configCipherErr)
	}
	if p.configCipher == nil {
		return nil, errMissingEncryptionKey
	}

	encrypted, err := p.configCipher.EncryptString(strings.TrimSpace(cfg.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt clerk api key: %w", err)
	}
	cfg.APIKey = encrypted

	normalized, err := json.Marshal(cfg) // #nosec G117 -- API key is encrypted before marshaling for persistence
	if err != nil {
		return nil, fmt.Errorf("failed to marshal clerk config: %w", err)
	}

	return normalized, nil
}

func (p *Provider) MigrateConfig(_ context.Context, fromVersion int32, configJSON []byte) (
	[]byte, error,
) {
	if fromVersion >= clerkLatestConfigVer {
		return configJSON, nil
	}
	// No migrations needed for v1 -> v1.
	return configJSON, nil
}

func (p *Provider) resolveConfig(configJSON []byte) (Config, error) {
	if len(configJSON) == 0 || string(configJSON) == "{}" {
		return Config{}, nil
	}

	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return Config{}, fmt.Errorf("invalid clerk config: %w", err)
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return cfg, nil
	}
	if !secrets.IsVersionedEncryptedSecret(strings.TrimSpace(cfg.APIKey)) {
		return cfg, nil
	}
	if p.configCipherErr != nil {
		return Config{}, fmt.Errorf("failed to initialize clerk config cipher: %w", p.configCipherErr)
	}
	if p.configCipher == nil {
		return Config{}, errMissingEncryptionKey
	}

	decrypted, err := p.configCipher.DecryptString(strings.TrimSpace(cfg.APIKey))
	if err != nil {
		return Config{}, fmt.Errorf("failed to decrypt clerk api key: %w", err)
	}
	cfg.APIKey = decrypted

	return cfg, nil
}

// ValidateWebhook verifies the Svix webhook signature.
// Clerk uses Svix for webhook delivery.
func (p *Provider) ValidateWebhook(
	_ context.Context,
	payload []byte,
	headers map[string]string,
	secret string,
) error {
	wh, err := svix.NewWebhook(secret)
	if err != nil {
		return fmt.Errorf("invalid svix webhook secret: %w", err)
	}

	svixHeaders := http.Header{}
	if msgID := headerGet(headers, svixIDHeader); msgID != "" {
		svixHeaders.Set("Svix-Id", msgID)
	}
	if timestamp := headerGet(headers, svixTimestampHeader); timestamp != "" {
		svixHeaders.Set("Svix-Timestamp", timestamp)
	}
	if signature := headerGet(headers, svixSignatureHeader); signature != "" {
		svixHeaders.Set("Svix-Signature", signature)
	}

	if verifyErr := wh.Verify(payload, svixHeaders); verifyErr != nil {
		return fmt.Errorf("webhook signature verification failed: %w", verifyErr)
	}

	return nil
}

// clerkWebhookEvent represents the top-level structure of a Clerk webhook payload.
type clerkWebhookEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (p *Provider) ExtractEventType(payload []byte) string {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ""
	}
	return envelope.Type
}

// clerkUserData represents the user data from a Clerk webhook.
type clerkUserData struct {
	ID             string              `json:"id"`
	EmailAddresses []clerkEmailAddress `json:"email_addresses"`
	FirstName      *string             `json:"first_name"`
	LastName       *string             `json:"last_name"`
	PrimaryEmailID *string             `json:"primary_email_address_id"`
}

type clerkEmailAddress struct {
	ID           string `json:"id"`
	EmailAddress string `json:"email_address"`
}

func (p *Provider) ParseEvent(_ context.Context, _ string, payload []byte) (any, error) {
	var event clerkWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse clerk webhook payload: %w", err)
	}
	return &event, nil
}

func (p *Provider) ToStandardsCommands(
	_ context.Context, _ string, event any,
) ([]provider.Command, error) {
	clerkEvent, ok := event.(*clerkWebhookEvent)
	if !ok {
		return nil, fmt.Errorf("expected *clerkWebhookEvent, got %T", event)
	}

	switch clerkEvent.Type {
	case "user.created", "user.updated":
		return p.handleUserEvent(clerkEvent)
	case "user.deleted":
		return p.handleUserDeletedEvent(clerkEvent)
	default:
		p.logger.Debug().Str(
			"event_type", clerkEvent.Type,
		).Msg("unhandled clerk event type, skipping")
		return nil, nil
	}
}

// ExecuteCommand executes a single canonical command produced by this provider.
func (p *Provider) ExecuteCommand(
	ctx context.Context,
	logger zerolog.Logger,
	instance *integration.Instance,
	cmd provider.Command,
) error {
	switch cmd.Type {
	case CommandUpsertUser:
		return p.executeUpsertUser(ctx, logger, instance, cmd.Data)
	case CommandDeleteUser:
		return p.executeDeleteUser(ctx, logger, instance, cmd.Data)
	default:
		provider.WriteAuditLog(ctx, logger, p.auditLogRepo, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionCommandTypeInvalid,
			Severity:              integration.AuditSeverityWarning,
			Message:               "Unknown command type skipped",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				"command_type": string(cmd.Type),
			}),
		})

		logger.Warn().
			Str("command_type", string(cmd.Type)).
			Msg("unknown command type, skipping")
		return nil
	}
}

func (p *Provider) handleUserEvent(event *clerkWebhookEvent) ([]provider.Command, error) {
	var userData clerkUserData
	if err := json.Unmarshal(event.Data, &userData); err != nil {
		return nil, fmt.Errorf("failed to parse clerk user data: %w", err)
	}

	email := p.extractPrimaryEmail(userData)
	name := buildFullName(userData.FirstName, userData.LastName)

	return []provider.Command{
		{
			Type: CommandUpsertUser,
			Data: UpsertUserData{
				ExternalID: userData.ID,
				Email:      email,
				Name:       name,
			},
		},
	}, nil
}

func (p *Provider) handleUserDeletedEvent(event *clerkWebhookEvent) (
	[]provider.Command, error,
) {
	var userData struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(event.Data, &userData); err != nil {
		return nil, fmt.Errorf("failed to parse clerk user deleted data: %w", err)
	}

	return []provider.Command{
		{
			Type: CommandDeleteUser,
			Data: DeleteUserData{
				ExternalID: userData.ID,
			},
		},
	}, nil
}

func (p *Provider) extractPrimaryEmail(user clerkUserData) string {
	if user.PrimaryEmailID != nil {
		primary := functional.Slice(user.EmailAddresses).FindFirst(func(ea clerkEmailAddress) bool {
			return ea.ID == *user.PrimaryEmailID
		})
		if primary.IsPresent() {
			return primary.Value().EmailAddress
		}
	}
	if len(user.EmailAddresses) > 0 {
		return user.EmailAddresses[0].EmailAddress
	}
	return ""
}

const fullNameParts = 2

func buildFullName(first, last *string) string {
	parts := make([]string, 0, fullNameParts)
	if first != nil && *first != "" {
		parts = append(parts, *first)
	}
	if last != nil && *last != "" {
		parts = append(parts, *last)
	}
	return strings.Join(parts, " ")
}

// headerGet performs case-insensitive header lookup.
func headerGet(headers map[string]string, key string) string {
	lowerKey := strings.ToLower(key)
	for k, v := range headers {
		if strings.ToLower(k) == lowerKey {
			return v
		}
	}
	return ""
}

func (p *Provider) WebhookEvents() []provider.WebhookEvent {
	return []provider.WebhookEvent{
		{
			Type:        string(events.ClerkUserCreated),
			Name:        "Clerk user created",
			Description: "Emitted when a product user is created from a Clerk webhook.",
		},
		{
			Type:        string(events.ClerkUserUpdated),
			Name:        "Clerk user updated",
			Description: "Emitted when a product user is updated from a Clerk webhook.",
		},
		{
			Type:        string(events.ClerkUserDeleted),
			Name:        "Clerk user deleted",
			Description: "Emitted when a product user is deleted from a Clerk webhook.",
		},
	}
}

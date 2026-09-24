package provider

import (
	"context"
	"errors"

	"anchor/internal/domain/integration"

	"github.com/rs/zerolog"
)

// ErrMessageRejected marks a permanent rejection of an outbound message by the
// remote mail system — an unauthorized sender, an undeliverable recipient, or
// rejected content. It signals a caller-input problem (a 4xx-class condition),
// not a transport or infrastructure fault, so the email domain maps it to a
// client error instead of a 5xx delivery failure. Providers wrap it around the
// underlying transport cause; that detail stays in server-side logs and is
// never serialized to the client.
var ErrMessageRejected = errors.New("outbound message rejected by provider")

// Provider is the base interface every integration plugin implements.
// It is intentionally domain-agnostic: provider identity + config lifecycle
// only. Behavioural concerns (inbound webhooks, outbound mail, reconciliation,
// ...) live in the optional capability interfaces below; callers must
// type-assert before invoking them.
type Provider interface {
	// Type returns the provider type identifier (e.g., "CLERK", "SMTP").
	Type() string

	// LatestConfigVersion returns the latest config schema version for this provider.
	LatestConfigVersion() int32

	// ValidateConfig validates that the given config JSON is valid for this provider.
	ValidateConfig(ctx context.Context, configJSON []byte) error

	// MigrateConfig migrates config JSON from an older version to the latest version.
	MigrateConfig(ctx context.Context, fromVersion int32, configJSON []byte) ([]byte, error)
}

// ConfigStorageProvider is implemented by providers that need to normalize or
// protect config data before persistence (for example, encrypting sensitive
// fields at rest).
type ConfigStorageProvider interface {
	Provider

	// PrepareConfigForStorage returns a sanitized config payload that is safe
	// to store in integration_instances.config_json.
	PrepareConfigForStorage(ctx context.Context, configJSON []byte) ([]byte, error)
}

// ConfigUpdateStorageProvider is implemented by providers that need the
// existing persisted config to safely prepare an update payload for storage.
// This is useful for write-only secrets that should be preserved when the
// update omits them.
type ConfigUpdateStorageProvider interface {
	ConfigStorageProvider

	// PrepareUpdatedConfigForStorage returns a sanitized config payload that is
	// safe to store for an update operation, with access to the currently
	// persisted config when secret fields need to be preserved.
	PrepareUpdatedConfigForStorage(ctx context.Context, currentConfigJSON []byte, configJSON []byte) ([]byte, error)
}

// ConnectionVerifier is implemented by providers that can actively probe a
// remote system to confirm the supplied configuration works (e.g. SMTP
// EHLO+AUTH, REST API ping). Used during instance create/update to flip the
// lifecycle state from CONFIGURING to ACTIVE.
type ConnectionVerifier interface {
	Provider

	// VerifyConnection performs a live health check against the integration's
	// remote system using the persisted config. Returns nil on success.
	VerifyConnection(ctx context.Context, instance *integration.Instance) error
}

// WebhookIngestor is implemented by providers that receive and process
// inbound webhook payloads from a third-party system (e.g. Clerk via Svix,
// GitHub, Stripe). Providers without this capability cannot be addressed by
// the webhook ingestion endpoint.
type WebhookIngestor interface {
	Provider

	// ValidateWebhook validates a webhook payload's signature using the
	// provided secret.
	ValidateWebhook(ctx context.Context, payload []byte, headers map[string]string, secret string) error

	// ExtractEventType returns the provider-specific event type identifier
	// from the raw webhook payload. It should return an empty string when not
	// present or when payload decoding fails.
	ExtractEventType(payload []byte) string

	// ParseEvent parses a raw webhook payload into a provider-specific event
	// structure.
	ParseEvent(ctx context.Context, eventType string, payload []byte) (any, error)

	// ToStandardsCommands converts a parsed event into canonical commands that
	// the integration service can execute.
	ToStandardsCommands(ctx context.Context, eventType string, event any) ([]Command, error)

	// ExecuteCommand executes a single canonical command produced by this
	// provider. The provider owns the full execution logic for its commands.
	ExecuteCommand(
		ctx context.Context,
		logger zerolog.Logger,
		instance *integration.Instance,
		cmd Command,
	) error
}

// Mailer is implemented by providers that can send transactional outbound
// email. The email domain resolves the product's email integration, asserts
// this capability, and dispatches a fully rendered OutboundMessage.
type Mailer interface {
	Provider

	// Send transmits a fully rendered message through the provider's transport
	// (SMTP, REST API, ...). The caller is responsible for rendering, recipient
	// validation, idempotency, and retry policy.
	Send(ctx context.Context, instance *integration.Instance, msg OutboundMessage) error

	// SenderIdentity returns the From address + display name configured on
	// the integration instance. Used by the email service to populate the
	// audit-trail record before dispatch, without forcing it to know the
	// provider-specific config schema.
	SenderIdentity(ctx context.Context, instance *integration.Instance) (Address, error)
}

// OutboundMessage is the canonical, provider-agnostic representation of a
// fully rendered email ready for transport.
type OutboundMessage struct {
	From      Address
	To        []Address
	ReplyTo   *Address
	Subject   string
	HTML      string
	Text      string
	Headers   map[string]string
	MessageID string
}

// Address is an RFC 5322 mailbox: an email address with an optional display name.
type Address struct {
	Email string
	Name  string
}

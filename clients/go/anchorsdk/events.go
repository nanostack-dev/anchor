package anchorsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	eventTypeOrganizationCreated        = "organization.created"
	eventTypeOrganizationUpdated        = "organization.updated"
	eventTypeOrganizationDeleted        = "organization.deleted"
	eventTypeMembershipCreated          = "organization.membership.created"
	eventTypeMembershipUpdated          = "organization.membership.updated"
	eventTypeMembershipDeleted          = "organization.membership.deleted"
	eventTypeWorkspaceCreated           = "workspace.created"
	eventTypeWorkspaceUpdated           = "workspace.updated"
	eventTypeWorkspaceDeleted           = "workspace.deleted"
	eventTypeOrganizationAPIKeyCreated  = "organization.api_key.created"
	eventTypeOrganizationAPIKeyUpdated  = "organization.api_key.updated"
	eventTypeOrganizationAPIKeyDeleted  = "organization.api_key.deleted"
	eventTypeProductUserCreated         = "product_user.created"
	eventTypeProductUserUpdated         = "product_user.updated"
	eventTypeProductUserDeleted         = "product_user.deleted"
	eventTypeOrganizationLicenseUpdated = "organization.license.updated"

	maxWebhookBodyBytes = 1 << 20
)

var (
	ErrWebhookSecret    = errors.New("anchorsdk: invalid webhook signing secret")
	ErrWebhookSignature = errors.New("anchorsdk: webhook signature verification failed")
	ErrWebhookPayload   = errors.New("anchorsdk: webhook payload is not a product event")
)

// Event is one Standard Webhooks product event after signature verification.
type Event struct {
	ID        string
	Type      string
	Timestamp time.Time
	Data      json.RawMessage
}

// Field returns a thin-id field from Data, or empty when the key is absent.
func (e Event) Field(key string) string {
	var fields map[string]string
	if err := json.Unmarshal(e.Data, &fields); err != nil {
		return ""
	}
	return fields[key]
}

type OrganizationCreated struct {
	OrganizationID string `json:"organization_id"`
}

type OrganizationUpdated struct {
	OrganizationID string `json:"organization_id"`
}

type OrganizationDeleted struct {
	OrganizationID string `json:"organization_id"`
}

type MembershipCreated struct {
	OrganizationID string `json:"organization_id"`
	ProductUserID  string `json:"product_user_id"`
}

type MembershipUpdated struct {
	OrganizationID string `json:"organization_id"`
	ProductUserID  string `json:"product_user_id"`
}

type MembershipDeleted struct {
	OrganizationID string `json:"organization_id"`
	ProductUserID  string `json:"product_user_id"`
}

type WorkspaceCreated struct {
	OrganizationID string `json:"organization_id"`
	WorkspaceID    string `json:"workspace_id"`
}

type WorkspaceUpdated struct {
	OrganizationID string `json:"organization_id"`
	WorkspaceID    string `json:"workspace_id"`
}

type WorkspaceDeleted struct {
	OrganizationID string `json:"organization_id"`
	WorkspaceID    string `json:"workspace_id"`
}

type OrganizationAPIKeyCreated struct {
	OrganizationID string `json:"organization_id"`
	APIKeyID       string `json:"api_key_id"`
}

type OrganizationAPIKeyUpdated struct {
	OrganizationID string `json:"organization_id"`
	APIKeyID       string `json:"api_key_id"`
}

type OrganizationAPIKeyDeleted struct {
	OrganizationID string `json:"organization_id"`
	APIKeyID       string `json:"api_key_id"`
}

type ProductUserCreated struct {
	ProductUserID string `json:"product_user_id"`
}

type ProductUserUpdated struct {
	ProductUserID string `json:"product_user_id"`
}

type ProductUserDeleted struct {
	ProductUserID string `json:"product_user_id"`
}

type OrganizationLicenseUpdated struct {
	OrganizationID string `json:"organization_id"`
}

type eventEnvelope struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// EventTypes is the product-event catalog a subscriber can handle.
func EventTypes() []string {
	return []string{
		eventTypeOrganizationCreated,
		eventTypeOrganizationUpdated,
		eventTypeOrganizationDeleted,
		eventTypeMembershipCreated,
		eventTypeMembershipUpdated,
		eventTypeMembershipDeleted,
		eventTypeWorkspaceCreated,
		eventTypeWorkspaceUpdated,
		eventTypeWorkspaceDeleted,
		eventTypeOrganizationAPIKeyCreated,
		eventTypeOrganizationAPIKeyUpdated,
		eventTypeOrganizationAPIKeyDeleted,
		eventTypeProductUserCreated,
		eventTypeProductUserUpdated,
		eventTypeProductUserDeleted,
		eventTypeOrganizationLicenseUpdated,
	}
}

// WebhookHandler verifies Standard Webhooks signatures and dispatches typed
// product events. It is an [http.Handler].
//
//	h, err := anchorsdk.Events(os.Getenv("ANCHOR_WEBHOOK_SECRET"))
//	h.OrganizationCreated(func(ctx context.Context, e anchorsdk.OrganizationCreated) error {
//	    return syncOrg(ctx, e.OrganizationID)
//	})
//	http.Handle("/webhooks/anchor", h)
type WebhookHandler struct {
	secret []byte

	onAny                        []func(context.Context, Event) error
	onOrganizationCreated        []func(context.Context, OrganizationCreated) error
	onOrganizationUpdated        []func(context.Context, OrganizationUpdated) error
	onOrganizationDeleted        []func(context.Context, OrganizationDeleted) error
	onMembershipCreated          []func(context.Context, MembershipCreated) error
	onMembershipUpdated          []func(context.Context, MembershipUpdated) error
	onMembershipDeleted          []func(context.Context, MembershipDeleted) error
	onWorkspaceCreated           []func(context.Context, WorkspaceCreated) error
	onWorkspaceUpdated           []func(context.Context, WorkspaceUpdated) error
	onWorkspaceDeleted           []func(context.Context, WorkspaceDeleted) error
	onOrganizationAPIKeyCreated  []func(context.Context, OrganizationAPIKeyCreated) error
	onOrganizationAPIKeyUpdated  []func(context.Context, OrganizationAPIKeyUpdated) error
	onOrganizationAPIKeyDeleted  []func(context.Context, OrganizationAPIKeyDeleted) error
	onProductUserCreated         []func(context.Context, ProductUserCreated) error
	onProductUserUpdated         []func(context.Context, ProductUserUpdated) error
	onProductUserDeleted         []func(context.Context, ProductUserDeleted) error
	onOrganizationLicenseUpdated []func(context.Context, OrganizationLicenseUpdated) error
}

// Events builds a [WebhookHandler] for the product signing secret (`whsec_...`).
func Events(signingSecret string) (*WebhookHandler, error) {
	return NewWebhookHandler(signingSecret)
}

// NewWebhookHandler is [Events].
func NewWebhookHandler(signingSecret string) (*WebhookHandler, error) {
	key, err := decodeSigningSecret(signingSecret)
	if err != nil {
		return nil, err
	}
	return &WebhookHandler{secret: key}, nil
}

func (h *WebhookHandler) OnAny(fn func(context.Context, Event) error) *WebhookHandler {
	h.onAny = append(h.onAny, fn)
	return h
}

func (h *WebhookHandler) OrganizationCreated(fn func(context.Context, OrganizationCreated) error) *WebhookHandler {
	h.onOrganizationCreated = append(h.onOrganizationCreated, fn)
	return h
}

func (h *WebhookHandler) OrganizationUpdated(fn func(context.Context, OrganizationUpdated) error) *WebhookHandler {
	h.onOrganizationUpdated = append(h.onOrganizationUpdated, fn)
	return h
}

func (h *WebhookHandler) OrganizationDeleted(fn func(context.Context, OrganizationDeleted) error) *WebhookHandler {
	h.onOrganizationDeleted = append(h.onOrganizationDeleted, fn)
	return h
}

func (h *WebhookHandler) MembershipCreated(fn func(context.Context, MembershipCreated) error) *WebhookHandler {
	h.onMembershipCreated = append(h.onMembershipCreated, fn)
	return h
}

func (h *WebhookHandler) MembershipUpdated(fn func(context.Context, MembershipUpdated) error) *WebhookHandler {
	h.onMembershipUpdated = append(h.onMembershipUpdated, fn)
	return h
}

func (h *WebhookHandler) MembershipDeleted(fn func(context.Context, MembershipDeleted) error) *WebhookHandler {
	h.onMembershipDeleted = append(h.onMembershipDeleted, fn)
	return h
}

func (h *WebhookHandler) WorkspaceCreated(fn func(context.Context, WorkspaceCreated) error) *WebhookHandler {
	h.onWorkspaceCreated = append(h.onWorkspaceCreated, fn)
	return h
}

func (h *WebhookHandler) WorkspaceUpdated(fn func(context.Context, WorkspaceUpdated) error) *WebhookHandler {
	h.onWorkspaceUpdated = append(h.onWorkspaceUpdated, fn)
	return h
}

func (h *WebhookHandler) WorkspaceDeleted(fn func(context.Context, WorkspaceDeleted) error) *WebhookHandler {
	h.onWorkspaceDeleted = append(h.onWorkspaceDeleted, fn)
	return h
}

func (h *WebhookHandler) OrganizationAPIKeyCreated(
	fn func(context.Context, OrganizationAPIKeyCreated) error,
) *WebhookHandler {
	h.onOrganizationAPIKeyCreated = append(h.onOrganizationAPIKeyCreated, fn)
	return h
}

func (h *WebhookHandler) OrganizationAPIKeyUpdated(
	fn func(context.Context, OrganizationAPIKeyUpdated) error,
) *WebhookHandler {
	h.onOrganizationAPIKeyUpdated = append(h.onOrganizationAPIKeyUpdated, fn)
	return h
}

func (h *WebhookHandler) OrganizationAPIKeyDeleted(
	fn func(context.Context, OrganizationAPIKeyDeleted) error,
) *WebhookHandler {
	h.onOrganizationAPIKeyDeleted = append(h.onOrganizationAPIKeyDeleted, fn)
	return h
}

func (h *WebhookHandler) ProductUserCreated(fn func(context.Context, ProductUserCreated) error) *WebhookHandler {
	h.onProductUserCreated = append(h.onProductUserCreated, fn)
	return h
}

func (h *WebhookHandler) ProductUserUpdated(fn func(context.Context, ProductUserUpdated) error) *WebhookHandler {
	h.onProductUserUpdated = append(h.onProductUserUpdated, fn)
	return h
}

func (h *WebhookHandler) ProductUserDeleted(fn func(context.Context, ProductUserDeleted) error) *WebhookHandler {
	h.onProductUserDeleted = append(h.onProductUserDeleted, fn)
	return h
}

func (h *WebhookHandler) OrganizationLicenseUpdated(
	fn func(context.Context, OrganizationLicenseUpdated) error,
) *WebhookHandler {
	h.onOrganizationLicenseUpdated = append(h.onOrganizationLicenseUpdated, fn)
	return h
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodyBytes+1))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	if len(body) > maxWebhookBodyBytes {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	msgID, err := verifySignature(h.secret, r.Header, body)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}
	event, err := decodeEvent(msgID, body)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if dispatchErr := h.dispatch(r.Context(), event); dispatchErr != nil {
		http.Error(w, "handler failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func decodeEvent(msgID string, body []byte) (Event, error) {
	var envelope eventEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return Event{}, fmt.Errorf("%w: %w", ErrWebhookPayload, err)
	}
	if envelope.Type == "" {
		return Event{}, ErrWebhookPayload
	}
	timestamp, err := time.Parse(time.RFC3339Nano, envelope.Timestamp)
	if err != nil {
		timestamp, err = time.Parse(time.RFC3339, envelope.Timestamp)
		if err != nil {
			return Event{}, fmt.Errorf("%w: timestamp", ErrWebhookPayload)
		}
	}
	return Event{
		ID:        msgID,
		Type:      envelope.Type,
		Timestamp: timestamp,
		Data:      envelope.Data,
	}, nil
}

func (h *WebhookHandler) dispatch(ctx context.Context, event Event) error {
	for _, fn := range h.onAny {
		if err := fn(ctx, event); err != nil {
			return err
		}
	}
	switch event.Type {
	case eventTypeOrganizationCreated:
		return dispatchTyped(ctx, event.Data, h.onOrganizationCreated)
	case eventTypeOrganizationUpdated:
		return dispatchTyped(ctx, event.Data, h.onOrganizationUpdated)
	case eventTypeOrganizationDeleted:
		return dispatchTyped(ctx, event.Data, h.onOrganizationDeleted)
	case eventTypeMembershipCreated:
		return dispatchTyped(ctx, event.Data, h.onMembershipCreated)
	case eventTypeMembershipUpdated:
		return dispatchTyped(ctx, event.Data, h.onMembershipUpdated)
	case eventTypeMembershipDeleted:
		return dispatchTyped(ctx, event.Data, h.onMembershipDeleted)
	case eventTypeWorkspaceCreated:
		return dispatchTyped(ctx, event.Data, h.onWorkspaceCreated)
	case eventTypeWorkspaceUpdated:
		return dispatchTyped(ctx, event.Data, h.onWorkspaceUpdated)
	case eventTypeWorkspaceDeleted:
		return dispatchTyped(ctx, event.Data, h.onWorkspaceDeleted)
	case eventTypeOrganizationAPIKeyCreated:
		return dispatchTyped(ctx, event.Data, h.onOrganizationAPIKeyCreated)
	case eventTypeOrganizationAPIKeyUpdated:
		return dispatchTyped(ctx, event.Data, h.onOrganizationAPIKeyUpdated)
	case eventTypeOrganizationAPIKeyDeleted:
		return dispatchTyped(ctx, event.Data, h.onOrganizationAPIKeyDeleted)
	case eventTypeProductUserCreated:
		return dispatchTyped(ctx, event.Data, h.onProductUserCreated)
	case eventTypeProductUserUpdated:
		return dispatchTyped(ctx, event.Data, h.onProductUserUpdated)
	case eventTypeProductUserDeleted:
		return dispatchTyped(ctx, event.Data, h.onProductUserDeleted)
	case eventTypeOrganizationLicenseUpdated:
		return dispatchTyped(ctx, event.Data, h.onOrganizationLicenseUpdated)
	default:
		return nil
	}
}

func dispatchTyped[T any](ctx context.Context, raw json.RawMessage, handlers []func(context.Context, T) error) error {
	if len(handlers) == 0 {
		return nil
	}
	var payload T
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("%w: %w", ErrWebhookPayload, err)
	}
	for _, fn := range handlers {
		if err := fn(ctx, payload); err != nil {
			return err
		}
	}
	return nil
}

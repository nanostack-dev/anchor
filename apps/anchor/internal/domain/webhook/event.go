package webhook

import (
	"encoding/json"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// Event is the outbox row: written in the same transaction as the business
// change that produced it, so an event exists if and only if that change
// committed.
type Event struct {
	ID             string
	ProductID      string
	OrganizationID *string
	EventType      string
	APIVersion     string
	// Payload is the envelope's `data` object, already serialized.
	Payload    json.RawMessage
	OccurredAt time.Time
	// TargetEndpointID restricts fan-out to a single endpoint. It is set only
	// for synthetic events (ping) and nil for ordinary broadcast events.
	TargetEndpointID *string
	CreatedAt        time.Time
}

// GenerateID sets the event's ID to a new prefixed KSUID.
func (e *Event) GenerateID() {
	e.ID = ids.MustNew("evt")
}

// Envelope is the wire shape delivered to receivers.
//
// It is deliberately thin: identifiers plus enough denormalized context for a
// receiver to decide whether it cares, after which the receiver re-reads the
// API for truth. Because delivery is unordered, a fat copied snapshot can be
// stale on arrival, and pushing full entitlement maps into third-party logs is
// a liability for an identity system.
type Envelope struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	APIVersion     string          `json:"api_version"`
	OccurredAt     time.Time       `json:"occurred_at"`
	ProductID      string          `json:"product_id"`
	OrganizationID *string         `json:"organization_id,omitempty"`
	Data           json.RawMessage `json:"data"`
}

// Envelope builds the wire representation of the event.
func (e *Event) Envelope() Envelope {
	data := e.Payload
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}

	return Envelope{
		ID:             e.ID,
		Type:           e.EventType,
		APIVersion:     e.APIVersion,
		OccurredAt:     e.OccurredAt.UTC(),
		ProductID:      e.ProductID,
		OrganizationID: e.OrganizationID,
		Data:           data,
	}
}

// LicenseChange describes a single field transition inside a license event's
// `changes` map.
type LicenseChange struct {
	Previous any `json:"previous"`
	New      any `json:"new"`
}

// LicenseEventData is the `data` object of a `license.*` event.
type LicenseEventData struct {
	LicenseID string                   `json:"license_id"`
	PlanKey   string                   `json:"plan_key,omitempty"`
	PlanID    string                   `json:"plan_id,omitempty"`
	Status    string                   `json:"status"`
	Changes   map[string]LicenseChange `json:"changes,omitempty"`
}

// PlanEventData is the `data` object of a `plan.updated` event.
type PlanEventData struct {
	PlanID  string `json:"plan_id"`
	PlanKey string `json:"plan_key"`
	Name    string `json:"name,omitempty"`
}

// PingEventData is the `data` object of a `ping` event.
type PingEventData struct {
	EndpointID string `json:"endpoint_id"`
	Message    string `json:"message"`
}

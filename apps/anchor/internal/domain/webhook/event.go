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
	// for synthetic test sends and nil for ordinary broadcast events.
	TargetEndpointID *string
	CreatedAt        time.Time
}

// GenerateID sets the event's ID to a new prefixed KSUID.
func (e *Event) GenerateID() {
	e.ID = ids.MustNew("evt")
}

// IsTest reports whether the event is a synthetic send rather than the record
// of a business change.
//
// Targeting is the marker, and it is exact rather than convenient: an event
// addressed at a single endpoint can only have come from the test-event
// sub-resource, because every business emit broadcasts to whoever subscribes.
// Should a future feature ever need to target a real event at one endpoint,
// this derivation must become a stored column instead.
func (e *Event) IsTest() bool {
	return e.TargetEndpointID != nil
}

// Envelope is the wire shape delivered to receivers.
//
// It is deliberately thin: identifiers plus enough denormalized context for a
// receiver to decide whether it cares, after which the receiver re-reads the
// API for truth. Because delivery is unordered, a fat copied snapshot can be
// stale on arrival, and pushing full entitlement maps into third-party logs is
// a liability for an identity system.
type Envelope struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	APIVersion string    `json:"api_version"`
	OccurredAt time.Time `json:"occurred_at"`
	// Test marks a send triggered from the admin UI's test surface rather than
	// a real business change. It is always present, never omitted: a receiver
	// must be able to assert `test == false` before acting on an event, and an
	// absent field would make "not a test" and "sent by an older Anchor"
	// indistinguishable.
	Test           bool            `json:"test"`
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
		Test:           e.IsTest(),
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

package integration

import (
	"encoding/json"
	"time"

	"github.com/nanostack-dev/shared/toolkit"
)

type EventStatus string

const (
	EventStatusPending    EventStatus = "PENDING"
	EventStatusProcessing EventStatus = "PROCESSING"
	EventStatusProcessed  EventStatus = "PROCESSED"
	EventStatusFailed     EventStatus = "FAILED"
)

type Event struct {
	ID                    string
	IntegrationInstanceID string
	ExternalEventID       string
	EventType             string
	PayloadJSON           json.RawMessage
	HeadersJSON           json.RawMessage
	Status                EventStatus
	Error                 *string
	ProcessedAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// GenerateID sets the event's ID to a new prefixed KSUID.
func (e *Event) GenerateID() {
	e.ID = toolkit.NewID("iev")
}

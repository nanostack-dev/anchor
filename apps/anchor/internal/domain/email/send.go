package email

import (
	"encoding/json"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// SendStatus is the lifecycle of a single send record.
type SendStatus string

const (
	// SendStatusQueued means the send has been accepted and is awaiting
	// dispatch by the email worker.
	SendStatusQueued SendStatus = "QUEUED"
	// SendStatusSending means the worker is currently delivering the message.
	SendStatusSending SendStatus = "SENDING"
	// SendStatusSent means the message was successfully handed off to the
	// SMTP provider (we do not yet track downstream delivery / open / click).
	SendStatusSent SendStatus = "SENT"
	// SendStatusFailed means the message failed permanently after retries
	// or was rejected for a hard reason (invalid template, etc.).
	SendStatusFailed SendStatus = "FAILED"
)

// SendRecord is the immutable audit row for every send attempt. Stored even
// when the send fails, so customers can always answer the question
// "did we email this user?".
//
// TemplateVersionID captures which DRAFT/PUBLISHED snapshot was actually
// rendered, so support can replay a send exactly as the recipient saw it
// even after the template has been re-published.
type SendRecord struct {
	ID                    string
	PlatformTenantID      string
	ProductID             string
	IntegrationInstanceID string
	TemplateID            *string
	TemplateVersionID     *string
	DedupeKey             *string
	ToAddress             string
	ToName                string
	FromAddress           string
	FromName              string
	Subject               string
	BodyHTML              string
	BodyText              string
	VariablesJSON         json.RawMessage
	MessageID             string
	Status                SendStatus
	Attempts              int32
	LastError             *string
	SentAt                *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// GenerateID sets the record's ID to a new prefixed KSUID.
func (s *SendRecord) GenerateID() {
	s.ID = ids.MustNew("esnd")
}

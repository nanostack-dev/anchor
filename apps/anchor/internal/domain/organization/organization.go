package organization

import (
	"encoding/json"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

type Organization struct {
	ID          string
	ProductID   string
	Name        string
	Description *string
	// MetadataJSON holds the caller-supplied key-value metadata as raw JSON.
	// Nil means the organization has no metadata stored.
	MetadataJSON json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GenerateID sets the organization's ID to a new prefixed KSUID.
func (o *Organization) GenerateID() {
	o.ID = ids.MustNew("org")
}

package organization

import (
	"encoding/json"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"anchor/internal/domain/license"
)

type Organization struct {
	ID          string
	ProductID   string
	Name        string
	Description *string
	// MetadataJSON holds the caller-supplied key-value metadata as raw JSON.
	// Nil means the organization has no metadata stored.
	MetadataJSON json.RawMessage
	// License is set only when the caller asked for it — on create, and on a
	// read including [IncludeLicense]. Nil otherwise, which says nothing about
	// whether the organization holds one.
	License   *license.OrganizationLicense
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GenerateID sets the organization's ID to a new prefixed KSUID.
func (o *Organization) GenerateID() {
	o.ID = ids.MustNew("org")
}

package organization

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

type Organization struct {
	ID          string
	ProductID   string
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// GenerateID sets the organization's ID to a new prefixed KSUID.
func (o *Organization) GenerateID() {
	o.ID = ids.MustNew("org")
}

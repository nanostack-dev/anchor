package tenant

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

type Status string

const (
	Active Status = "ACTIVE"
)

type PlatformTenant struct {
	ID        string
	Name      string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GenerateID sets the platform tenant's ID to a new prefixed KSUID.
func (t *PlatformTenant) GenerateID() {
	t.ID = ids.MustNew("tenant")
}

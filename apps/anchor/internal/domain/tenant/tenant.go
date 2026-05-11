package tenant

import (
	"time"

	"github.com/nanostack-dev/shared/toolkit"
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
	t.ID = toolkit.NewID("tenant")
}

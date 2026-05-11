package invitation

import (
	"time"

	"github.com/nanostack-dev/shared/toolkit"
)

type PlatformInvitation struct {
	ID               string
	Code             string
	Email            string
	PlatformTenantID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// GenerateID sets the invitation's ID to a new prefixed KSUID.
func (i *PlatformInvitation) GenerateID() {
	i.ID = toolkit.NewID("pinv")
}

package invitation

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
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
	i.ID = ids.MustNew("pinv")
}

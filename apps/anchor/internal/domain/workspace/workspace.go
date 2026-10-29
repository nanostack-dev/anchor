package workspace

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

type Workspace struct {
	ID             string
	OrganizationID string
	Name           string
	Description    *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// GenerateID sets the workspace's ID to a new prefixed KSUID.
func (w *Workspace) GenerateID() {
	w.ID = ids.MustNew("ws")
}

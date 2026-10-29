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

// Membership is a row of workspace_memberships: a member holding a product
// role on a workspace. No route writes them; the workspace migration moves
// each one it can into a role assignment (ADR-0022 A6, D4).
type Membership struct {
	WorkspaceID   string
	ProductUserID string
	RoleID        string
}

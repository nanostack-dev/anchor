// Package organizationrole holds the roles each Organization owns (ADR-0022
// D8): a copy of each of its Product's role templates, and later its own
// custom roles.
package organizationrole

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

type Role struct {
	// RowID is the role's own row; memberships point at it. A copy is
	// addressed by its template's ID instead, see PublicID.
	RowID          string
	ProductID      string
	OrganizationID string
	TemplateID     *string
	Key            string
	Name           string
	Description    string
	ResourceTypeID *string
	Permissions    []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PublicID is the ID callers see and send: a copy keeps its template's ID
// inside its Organization, so the role IDs released clients hold keep
// working; a template or a custom role has its own.
func (r Role) PublicID() string {
	if r.TemplateID != nil {
		return *r.TemplateID
	}
	return r.RowID
}

// NewBodyRoleNotFoundError answers a role_id in a request body that names no
// role of the Organization and no role template of its Product. The code
// differs from the 404 a role_id path parameter answers: one code at two
// statuses breaks a client that switches on code alone.
func NewBodyRoleNotFoundError(roleID string) *fault.Error {
	return fault.BadRequest(
		"ROLE_NOT_FOUND_IN_REQUEST",
		"This product has no role with that identifier.",
	).Metadata(map[string]any{"role_id": roleID})
}

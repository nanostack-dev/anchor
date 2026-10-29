// Package source holds Anchor's sources: each syncs one slice of
// Anchor's tables into every Product's OpenFGA store, through the syncer
// (ADR-0022 E3 and E5). Each owns a disjoint set of tuples:
//
//	Source                     Key           Tuples it owns
//	organization_assignments   Organization  organization:O # assignment, role_assignment:O.R # role and # assignee
//	role_grants                role row      role:R # grants_<permission>
//	organization_api_keys      Organization  organization:O # can_<permission> @ api_key:K
//	resource_parents           resource      rt_<slug>:<row> # parent
//
// Catalog is the model source: a change to a resource type or a permission
// rewrites the Product's model and resyncs every source.
//
// To sync a new slice of the tables, add a type here that implements
// fgasync.Source, owns tuples no other source owns, and handles its
// events in Stale; then register it in authz.NewModule and remove its events
// from WithoutAccess. Stale lists events rather than switching over every
// event type: TestEveryEventIsClassified fails until each event type is either
// claimed by a source or the catalog, or listed in WithoutAccess.
package source

import (
	"database/sql"
	"fmt"
	"slices"
	"time"

	"anchor/internal/events"
)

// WithoutAccess lists the events no source syncs, so a new event type
// must be classified before the suite passes.
func WithoutAccess() []events.Type {
	return []events.Type{
		// Names and licenses hold no access.
		events.OrganizationCreated, events.OrganizationUpdated, events.OrganizationLicenseUpdated,
		// An accepted invitation adds its member through MembershipCreated.
		events.OrganizationInvitationCreated, events.OrganizationInvitationUpdated,
		events.OrganizationInvitationDeleted, events.OrganizationInvitationAccepted,
		// Workspaces are not modelled yet.
		events.WorkspaceCreated, events.WorkspaceUpdated, events.WorkspaceDeleted,
		// A product user holds access only through memberships.
		events.ProductUserCreated, events.ProductUserUpdated,
		// A deleted Clerk user also emits ProductUserDeleted.
		events.ClerkUserCreated, events.ClerkUserUpdated, events.ClerkUserDeleted,
	}
}

// Tables reads the tables the sources sync.
type Tables struct {
	db  *sql.DB
	now func() time.Time
}

func NewTables(db *sql.DB) Tables {
	return Tables{db: db, now: time.Now}
}

func isOneOf(event events.Event, types ...events.Type) bool {
	return slices.Contains(types, event.Type)
}

// keyOf returns the event's field that names the stale key.
func keyOf(event events.Event, field string) ([]string, error) {
	key := event.Data[field]
	if key == "" {
		return nil, fmt.Errorf("%s event without %s", event.Type, field)
	}
	return []string{key}, nil
}

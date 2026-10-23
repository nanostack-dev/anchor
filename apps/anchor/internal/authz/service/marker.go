package service

import (
	"context"

	"anchor/internal/authz/repository"
	"anchor/internal/events"
)

// Marker records which part of the relationship store an event makes stale.
// It runs inside the emitting transaction, so the mark and the write commit
// together and no committed change can go unsynced.
type Marker struct {
	repo repository.Repository
}

func NewMarker(repo repository.Repository) *Marker {
	return &Marker{repo: repo}
}

func (m *Marker) ObserveTx(ctx context.Context, event events.Event) error {
	switch stalenessOf(event.Type) {
	case staleProduct:
		return m.repo.Mark(ctx, event.ProductID, repository.ProductWide)
	case staleOrganization:
		if organizationID := event.Data[events.FieldOrganizationID]; organizationID != "" {
			return m.repo.Mark(ctx, event.ProductID, organizationID)
		}
		return nil
	case staleNothing:
		return nil
	}
	return nil
}

type staleness int

const (
	staleNothing staleness = iota
	// staleOrganization: the tuples of the organization the event names change.
	staleOrganization
	// staleProduct: the model changes, or tuples in organizations the event does
	// not name, as when a deleted product user loses every membership by cascade.
	staleProduct
)

// stalenessOf lists every event type, so a new type fails the exhaustive lint
// until someone decides whether it changes access.
func stalenessOf(eventType events.Type) staleness {
	switch eventType {
	case events.OrganizationCreated, events.OrganizationDeleted,
		events.MembershipCreated, events.MembershipUpdated, events.MembershipDeleted,
		events.WorkspaceCreated, events.WorkspaceDeleted,
		events.WorkspaceMembershipCreated, events.WorkspaceMembershipUpdated, events.WorkspaceMembershipDeleted,
		events.OrganizationAPIKeyCreated, events.OrganizationAPIKeyUpdated, events.OrganizationAPIKeyDeleted:
		return staleOrganization
	case events.ProductRoleCreated, events.ProductRoleUpdated, events.ProductRoleDeleted,
		events.ProductResourcePermissionCreated, events.ProductResourcePermissionUpdated,
		events.ProductResourcePermissionDeleted,
		events.ProductUserDeleted:
		return staleProduct
	case events.OrganizationUpdated, events.WorkspaceUpdated,
		events.OrganizationInvitationCreated, events.OrganizationInvitationUpdated,
		events.OrganizationInvitationDeleted, events.OrganizationInvitationAccepted,
		events.ProductUserCreated, events.ProductUserUpdated,
		events.OrganizationLicenseUpdated,
		events.ClerkUserCreated, events.ClerkUserUpdated, events.ClerkUserDeleted:
		// Names, invitations and licenses hold no access. An accepted invitation
		// adds its member through MembershipCreated; a Clerk delete also emits
		// ProductUserDeleted.
		return staleNothing
	}
	return staleNothing
}

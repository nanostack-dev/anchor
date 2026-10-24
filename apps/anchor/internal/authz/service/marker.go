package service

import (
	"context"
	"fmt"

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
	scopes, err := staleScopes(event)
	if err != nil {
		return err
	}
	for _, scope := range scopes {
		if markErr := m.repo.Mark(ctx, event.ProductID, scope); markErr != nil {
			return markErr
		}
	}
	return nil
}

// staleScopes lists every event type, so a new type fails the exhaustive lint
// until someone decides whether it changes access.
func staleScopes(event events.Event) ([]string, error) {
	switch event.Type {
	case events.OrganizationCreated, events.OrganizationDeleted,
		events.MembershipCreated, events.MembershipUpdated, events.MembershipDeleted,
		events.WorkspaceCreated, events.WorkspaceDeleted,
		events.WorkspaceMembershipCreated, events.WorkspaceMembershipUpdated, events.WorkspaceMembershipDeleted,
		events.OrganizationAPIKeyCreated, events.OrganizationAPIKeyUpdated, events.OrganizationAPIKeyDeleted:
		organizationID := event.Data[events.FieldOrganizationID]
		if organizationID == "" {
			return nil, fmt.Errorf("authz: %s event without %s", event.Type, events.FieldOrganizationID)
		}
		return []string{organizationID}, nil
	case events.ProductRoleCreated, events.ProductRoleUpdated,
		events.ProductResourcePermissionCreated, events.ProductResourcePermissionUpdated:
		return []string{repository.ModelScope}, nil
	case events.ProductRoleDeleted, events.ProductResourcePermissionDeleted:
		// The new model drops the relation, so leftover tuples grant nothing;
		// the product sweep removes them.
		return []string{repository.ModelScope, repository.ProductScope}, nil
	case events.ProductUserDeleted:
		// Its memberships are gone by cascade, and evaluation refuses an unknown
		// product user before it reads the store.
		return []string{repository.ProductScope}, nil
	case events.OrganizationUpdated, events.WorkspaceUpdated,
		events.OrganizationInvitationCreated, events.OrganizationInvitationUpdated,
		events.OrganizationInvitationDeleted, events.OrganizationInvitationAccepted,
		events.ProductUserCreated, events.ProductUserUpdated,
		events.OrganizationLicenseUpdated,
		events.ClerkUserCreated, events.ClerkUserUpdated, events.ClerkUserDeleted:
		// Names, invitations and licenses hold no access. An accepted invitation
		// adds its member through MembershipCreated; a Clerk delete also emits
		// ProductUserDeleted.
		return nil, nil
	}
	return nil, nil
}

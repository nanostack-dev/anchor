package service

import (
	"context"
	"fmt"
	"sync"

	"anchor/internal/authz/repository"
	"anchor/internal/events"
)

// Marker records which part of the relationship store an event makes stale.
// It runs inside the emitting transaction, so the mark and the write commit
// together and no committed change goes unsynced. It also tells the request,
// when one collects them, which Organizations to replay once it commits.
type Marker struct {
	repo repository.Repository
}

func NewMarker(repo repository.Repository) *Marker {
	return &Marker{repo: repo}
}

func (m *Marker) ObserveTx(ctx context.Context, event events.Event) error {
	scope, stale, err := staleScope(event)
	if err != nil || !stale {
		return err
	}
	if err = m.repo.Mark(ctx, event.ProductID, scope); err != nil {
		return err
	}
	if written := writesFrom(ctx); written != nil {
		written.add(event.ProductID, scope)
	}
	return nil
}

// staleScope lists every event type, so a new one fails the exhaustive lint
// until someone decides whether it changes access.
func staleScope(event events.Event) (string, bool, error) {
	switch event.Type {
	case events.OrganizationCreated, events.OrganizationDeleted,
		events.MembershipCreated, events.MembershipUpdated, events.MembershipDeleted,
		events.OrganizationAPIKeyCreated, events.OrganizationAPIKeyUpdated, events.OrganizationAPIKeyDeleted,
		events.OrganizationRoleCreated, events.OrganizationRoleUpdated, events.OrganizationRoleDeleted:
		organizationID := event.Data[events.FieldOrganizationID]
		if organizationID == "" {
			return "", false, fmt.Errorf("authz: %s event without %s", event.Type, events.FieldOrganizationID)
		}
		return organizationID, true, nil
	case events.ProductRoleCreated, events.ProductRoleUpdated, events.ProductRoleDeleted,
		events.ProductResourcePermissionCreated, events.ProductResourcePermissionUpdated,
		events.ProductResourcePermissionDeleted,
		events.ProductResourceTypeCreated, events.ProductResourceTypeUpdated, events.ProductResourceTypeDeleted,
		events.ProductUserDeleted:
		// A catalog or managed role change reaches every Organization and the model;
		// a deleted product user leaves memberships cascaded without events.
		return repository.ProductScope, true, nil
	case events.OrganizationUpdated,
		events.OrganizationInvitationCreated, events.OrganizationInvitationUpdated,
		events.OrganizationInvitationDeleted, events.OrganizationInvitationAccepted,
		events.WorkspaceCreated, events.WorkspaceUpdated, events.WorkspaceDeleted,
		events.ProductUserCreated, events.ProductUserUpdated,
		events.OrganizationLicenseUpdated,
		events.ClerkUserCreated, events.ClerkUserUpdated, events.ClerkUserDeleted:
		// Names, invitations, workspaces and licenses hold no access the store
		// mirrors yet. An accepted invitation adds its member through
		// MembershipCreated; a deleted Clerk user also emits ProductUserDeleted.
		return "", false, nil
	}
	return "", false, nil
}

// written collects the Organizations a request's writes marked.
type written struct {
	mu   sync.Mutex
	keys map[organizationKey]struct{}
}

type organizationKey struct {
	productID      string
	organizationID string
}

type writtenKey struct{}

// CollectWrites returns a context in which marks are also collected, and the
// function that lists them once the request's transaction has committed.
func CollectWrites(ctx context.Context) (context.Context, func() []OrganizationRef) {
	collected := &written{keys: map[organizationKey]struct{}{}}
	return context.WithValue(ctx, writtenKey{}, collected), collected.list
}

// OrganizationRef names one Organization of one Product, or the whole Product
// when OrganizationID is repository.ProductScope.
type OrganizationRef struct {
	ProductID      string
	OrganizationID string
}

func writesFrom(ctx context.Context) *written {
	collected, _ := ctx.Value(writtenKey{}).(*written)
	return collected
}

func (w *written) add(productID, organizationID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.keys[organizationKey{productID: productID, organizationID: organizationID}] = struct{}{}
}

func (w *written) list() []OrganizationRef {
	w.mu.Lock()
	defer w.mu.Unlock()
	refs := make([]OrganizationRef, 0, len(w.keys))
	for key := range w.keys {
		refs = append(refs, OrganizationRef{ProductID: key.productID, OrganizationID: key.organizationID})
	}
	return refs
}

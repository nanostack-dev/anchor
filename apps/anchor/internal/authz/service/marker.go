package service

import (
	"context"
	"fmt"
	"sync"

	"anchor/internal/authz/model"
	"anchor/internal/authz/repository"
	"anchor/internal/events"
)

// Marker records which part of the relationship store an event makes stale.
// It runs inside the emitting transaction, so the mark and the write commit
// together and no committed change goes unsynced. It also tells the request,
// when one collects them, which Organizations and resource objects to replay
// once it commits.
type Marker struct {
	repo repository.Repository
}

func NewMarker(repo repository.Repository) *Marker {
	return &Marker{repo: repo}
}

func (m *Marker) ObserveTx(ctx context.Context, event events.Event) error {
	target, stale, err := staleScope(event)
	if err != nil || !stale {
		return err
	}
	if target.object != "" {
		if err = m.repo.MarkResource(ctx, event.ProductID, target.organizationID, target.object); err != nil {
			return err
		}
		if written := writesFrom(ctx); written != nil {
			written.addResource(event.ProductID, target.object)
		}
		return nil
	}
	if err = m.repo.Mark(ctx, event.ProductID, target.organizationID); err != nil {
		return err
	}
	if written := writesFrom(ctx); written != nil {
		written.add(event.ProductID, target.organizationID)
	}
	return nil
}

// staleTarget is what an event makes stale: an Organization, the whole
// Product (repository.ProductScope), or one resource object of an
// Organization.
type staleTarget struct {
	organizationID string
	object         string
}

// staleScope lists every event type, so a new one fails the exhaustive lint
// until someone decides whether it changes access.
func staleScope(event events.Event) (staleTarget, bool, error) {
	switch event.Type {
	case events.OrganizationCreated, events.OrganizationDeleted,
		events.MembershipCreated, events.MembershipUpdated, events.MembershipDeleted,
		events.OrganizationAPIKeyCreated, events.OrganizationAPIKeyUpdated, events.OrganizationAPIKeyDeleted,
		events.OrganizationRoleCreated, events.OrganizationRoleUpdated, events.OrganizationRoleDeleted:
		organizationID, err := required(event, events.FieldOrganizationID)
		return staleTarget{organizationID: organizationID}, err == nil, err
	case events.ResourceCreated, events.ResourceUpdated, events.ResourceDeleted:
		// A resource write changes its own object only: a move rewrites its
		// parent tuple, and its children keep theirs.
		organizationID, err := required(event, events.FieldOrganizationID)
		if err != nil {
			return staleTarget{}, false, err
		}
		slug, err := required(event, events.FieldResourceType)
		if err != nil {
			return staleTarget{}, false, err
		}
		rowID, err := required(event, events.FieldResourceRowID)
		if err != nil {
			return staleTarget{}, false, err
		}
		return staleTarget{organizationID: organizationID, object: model.ResourceObject(slug, rowID)}, true, nil
	case events.ProductRoleCreated, events.ProductRoleUpdated, events.ProductRoleDeleted,
		events.ProductResourcePermissionCreated, events.ProductResourcePermissionUpdated,
		events.ProductResourcePermissionDeleted,
		events.ProductResourceTypeCreated, events.ProductResourceTypeUpdated, events.ProductResourceTypeDeleted,
		events.ProductUserDeleted:
		// A catalog change reaches every Organization's copies and the model;
		// a deleted product user leaves memberships cascaded without events.
		return staleTarget{organizationID: repository.ProductScope}, true, nil
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
		return staleTarget{}, false, nil
	}
	return staleTarget{}, false, nil
}

func required(event events.Event, field string) (string, error) {
	value := event.Data[field]
	if value == "" {
		return "", fmt.Errorf("authz: %s event without %s", event.Type, field)
	}
	return value, nil
}

// written collects the Organizations and resource objects a request's
// writes marked. Resource objects keep the order they were marked in.
type written struct {
	mu        sync.Mutex
	keys      map[organizationKey]struct{}
	resources []ResourceRef
	seen      map[ResourceRef]struct{}
}

type organizationKey struct {
	productID      string
	organizationID string
}

type writtenKey struct{}

// CollectWrites returns a context in which marks are also collected, and the
// function that lists the marked Organizations once the request's
// transaction has committed. WrittenResources lists the marked resource
// objects.
func CollectWrites(ctx context.Context) (context.Context, func() []OrganizationRef) {
	collected := &written{keys: map[organizationKey]struct{}{}, seen: map[ResourceRef]struct{}{}}
	return context.WithValue(ctx, writtenKey{}, collected), collected.list
}

// WrittenResources lists the resource objects marked in a context from
// CollectWrites, in the order they were first marked.
func WrittenResources(ctx context.Context) []ResourceRef {
	collected := writesFrom(ctx)
	if collected == nil {
		return nil
	}
	collected.mu.Lock()
	defer collected.mu.Unlock()
	return append([]ResourceRef{}, collected.resources...)
}

// OrganizationRef names one Organization of one Product, or the whole Product
// when OrganizationID is repository.ProductScope.
type OrganizationRef struct {
	ProductID      string
	OrganizationID string
}

// ResourceRef names one resource object in one Product's store.
type ResourceRef struct {
	ProductID string
	Object    string
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

func (w *written) addResource(productID, object string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	ref := ResourceRef{ProductID: productID, Object: object}
	if _, ok := w.seen[ref]; !ok {
		w.seen[ref] = struct{}{}
		w.resources = append(w.resources, ref)
	}
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

package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

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
	found, err := staleScope(event)
	if err != nil {
		return err
	}
	scope, stale := found.Get()
	if !stale {
		return nil
	}
	if err = m.repo.Mark(ctx, event.ProductID, scope); err != nil {
		return err
	}
	if collector := collectorFrom(ctx); collector != nil {
		collector.add(OrganizationRef{ProductID: event.ProductID, OrganizationID: scope})
	}
	return nil
}

// staleScope names what the event makes stale: an Organization, ProductScope,
// or nothing. It lists every event type, so the exhaustive lint makes a new
// one decide whether it changes access.
func staleScope(event events.Event) (functional.Option[string], error) {
	switch event.Type {
	case events.OrganizationCreated, events.OrganizationDeleted,
		events.MembershipCreated, events.MembershipUpdated, events.MembershipDeleted,
		events.OrganizationAPIKeyCreated, events.OrganizationAPIKeyUpdated, events.OrganizationAPIKeyDeleted,
		events.OrganizationRoleCreated, events.OrganizationRoleUpdated, events.OrganizationRoleDeleted:
		organizationID := event.Data[events.FieldOrganizationID]
		if organizationID == "" {
			return functional.None[string](), fmt.Errorf(
				"authz: %s event without %s", event.Type, events.FieldOrganizationID,
			)
		}
		return functional.Some(organizationID), nil
	case events.ProductRoleCreated, events.ProductRoleUpdated, events.ProductRoleDeleted,
		events.ProductResourcePermissionCreated, events.ProductResourcePermissionUpdated,
		events.ProductResourcePermissionDeleted,
		events.ProductResourceTypeCreated, events.ProductResourceTypeUpdated, events.ProductResourceTypeDeleted,
		events.ProductUserDeleted:
		// A catalog or managed role change reaches every Organization and the model;
		// a deleted product user leaves memberships cascaded without events.
		return functional.Some(repository.ProductScope), nil
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
	}
	return functional.None[string](), nil
}

// markCollector collects the Organizations a request's writes marked.
type markCollector struct {
	mu   sync.Mutex
	refs map[OrganizationRef]struct{}
}

type collectorKey struct{}

// CollectMarks returns a context in which marks are also collected, and the
// function that lists them once the request's transaction has committed.
func CollectMarks(ctx context.Context) (context.Context, func() []OrganizationRef) {
	collector := &markCollector{refs: map[OrganizationRef]struct{}{}}
	return context.WithValue(ctx, collectorKey{}, collector), collector.list
}

// OrganizationRef names one Organization of one Product, or the whole Product
// when OrganizationID is repository.ProductScope.
type OrganizationRef struct {
	ProductID      string
	OrganizationID string
}

func (r OrganizationRef) CoversProduct() bool { return r.OrganizationID == repository.ProductScope }

func collectorFrom(ctx context.Context) *markCollector {
	collector, _ := ctx.Value(collectorKey{}).(*markCollector)
	return collector
}

func (c *markCollector) add(ref OrganizationRef) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refs[ref] = struct{}{}
}

func (c *markCollector) list() []OrganizationRef {
	c.mu.Lock()
	defer c.mu.Unlock()
	refs := make([]OrganizationRef, 0, len(c.refs))
	for ref := range c.refs {
		refs = append(refs, ref)
	}
	return refs
}
